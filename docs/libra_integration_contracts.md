# Libra 통합 계약 및 사전 합의

> 상태: 초안 v0.1  
> 작성일: 2026-07-18  
> 목적: A·B·C가 독립적으로 구현한 기능을 결합할 때 데이터의 의미와 형태가 달라지는 문제를 방지한다.

## 1. 문서의 역할

사전 합의는 코드 소유권을 정하는 일이 아니라 계층과 팀 사이에서 주고받는 데이터의 의미와 형태를 정하는 일이다.

```text
A: 파일시스템을 탐색하고 사실을 전달한다.
B: 파일과 설정을 해석해 프로젝트·리소스·관계를 만든다.
C: application 결과를 CLI와 JSON으로 표현한다.
```

모든 계약은 다음 상태 중 하나를 가진다.

| 상태 | 의미 |
|---|---|
| `CONFIRMED` | 현재 코드와 문서가 이미 일치하며 그대로 사용한다. |
| `ADOPTED` | 이번 합의에서 채택했고 코드에도 반영했다. |
| `DECISION_REQUIRED` | 공용 domain, DB schema, CLI 계약 변경이 필요해 팀 합의 전에는 구현하지 않는다. |
| `PLANNED` | 방향은 합의 가능하지만 구현 순서상 후속 작업으로 남아 있다. |

## 2. 계약 현황 요약

| 번호 | 계약 | 상태 | 현재 결론 |
|---:|---|---|---|
| 1 | 프로젝트 단위 | `CONFIRMED` | `.sln`은 Workspace, `.vcxproj`·`.csproj` 등은 BuildProject다. |
| 2 | 독립 BuildProject | `CONFIRMED` | Workspace에 속하지 않은 BuildProject도 저장한다. |
| 3 | 프로젝트 고유성 | `DECISION_REQUIRED` | `normalized path + project type`을 후보 키로 사용한다. |
| 4 | 경로 정규화 | `ADOPTED` | 모든 계층이 `internal/pathutil.Normalize`를 사용한다. |
| 5 | A→B 전달 값 | `CONFIRMED` | A는 의미를 해석하지 않은 `scanner.Entry`를 전달한다. |
| 6 | 오류 분류 | `PLANNED` | recoverable/terminal 원칙은 확정, 공용 `ScanIssue` 형태는 추가 합의가 필요하다. |
| 7 | 프로젝트 DB 저장 | `DECISION_REQUIRED` | normalized path 기반 upsert/scan 단위 교체 중 하나를 결정해야 한다. |
| 8 | 최종 ScanResult | `DECISION_REQUIRED` | `scanner.Result`, `app.ScanResult`, `output.ScanView`를 분리한다. |
| 9 | 진행률 | `DECISION_REQUIRED` | application 계층의 `ProgressEvent` callback을 사용한다. |
| 10 | CLI·JSON | `DECISION_REQUIRED` | 결과는 stdout, 진행률·경고는 stderr로 분리한다. |

## 3. 프로젝트와 Workspace 계약

### 3.1 의미

```text
Workspace
└─ Game.sln
   ├─ Client.vcxproj (BuildProject)
   └─ Server.vcxproj (BuildProject)
```

- `.sln`은 사용자가 보는 작업공간이며 직접 SDK 의존성을 갖지 않는다.
- `.vcxproj`, `.csproj`, Node 프로젝트, 독립 Git 프로젝트는 직접 분석 가능한 BuildProject다.
- SDK와 도구 의존성은 Workspace가 아니라 BuildProject에 연결한다.
- 하나의 BuildProject가 여러 Workspace에 포함될 수 있다.
- Workspace에 포함되지 않은 BuildProject도 독립 프로젝트로 등록한다.
- 디렉터리 논리 크기는 실제 디렉터리를 기준으로 한 번만 계산한다.

### 3.2 현재 구현

`internal/domain/project.go`에는 다음 모델이 이미 존재한다.

```go
type Workspace struct { ... }
type BuildProject struct { ... }
type WorkspaceProject struct {
    WorkspaceID    string
    BuildProjectID string
}
```

`internal/adapter/msbuild`도 Workspace parser와 BuildProject parser를 분리한다.

### 3.3 남은 간극

현재 DB에는 Workspace 및 WorkspaceProject 전용 테이블이 없다. `dependencies` 테이블을 재사용할지 전용 테이블을 추가할지는 schema 변경이므로 팀 합의와 migration review가 필요하다.

권장 관계 이름은 다음과 같다.

```text
SOLUTION_CONTAINS_PROJECT
```

## 4. 경로와 프로젝트 ID 계약

### 4.1 표시 경로와 비교 경로

사용자에게 보여줄 경로와 DB 중복 제거에 사용할 경로는 목적이 다르다.

```text
DisplayPath:    D:\Game\Client
NormalizedPath: d:\game\client
```

권장 모델은 다음과 같다.

```go
type BuildProject struct {
    // 기존 필드 생략
    DisplayPath    string
    NormalizedPath string
}
```

이 필드 변경은 domain과 DB mapping에 영향을 주므로 아직 적용하지 않는다.

### 4.2 공용 정규화 함수

모든 팀은 독자적인 경로 정규화 함수를 만들지 않고 다음 함수만 사용한다.

```go
normalized, err := pathutil.Normalize(path)
```

표시용 절대 경로가 필요하면 대소문자를 보존하는 다음 함수를 사용한다.

```go
displayPath, err := pathutil.Absolute(path)
```

계약:

- 빈 경로는 오류다.
- 상대 경로는 절대 경로로 바꾼다.
- `.`과 `..`을 정리한다.
- 현재 OS의 기본 구분자로 정리한다.
- Windows에서는 비교 키를 소문자로 만든다.
- macOS 등 비 Windows 환경에서는 대소문자를 보존한다.
- symlink와 junction은 해석하지 않는다.
- 표시용 원본 경로를 대체하는 용도로 사용하지 않는다.

### 4.3 프로젝트 고유성

제안하는 프로젝트 고유성 키는 다음과 같다.

```text
normalized path + project type
```

동일 경로라도 서로 다른 project type을 별도 프로젝트로 볼 것인지 최종 합의가 필요하다. 합의 전에는 ID 생성 알고리즘을 추가하지 않는다.

결정할 항목:

1. ID를 hash로 만들지 UUID로 만들지
2. marker 파일 경로와 project root 중 무엇을 identity path로 쓸지
3. Git fallback project와 언어별 project가 같은 root에 있을 때 하나로 합칠지

## 5. A→B 파일 전달 계약

A의 scanner는 파일시스템의 사실만 전달한다.

```go
type Entry struct {
    Path       string
    Kind       EntryKind
    Size       int64
    Mode       fs.FileMode
    ModifiedAt time.Time
}
```

A가 판단하지 않는 항목:

- 프로젝트 여부
- 프로젝트 종류
- SDK 버전
- 재생성 가능성
- cleanup 위험도

B는 `scanner.Entry`를 받아 marker와 파일 내용을 해석한다. 디렉터리 entry도 `.git`, `node_modules`, `bin`, `obj`, `build` 탐지에 필요하므로 전달한다.

권장 detector 계약:

```go
type Detector interface {
    Observe(ctx context.Context, entry scanner.Entry) ([]domain.BuildProject, error)
}
```

현재 adapter별 detector 계약이 서로 다르므로 공용 interface 도입 여부는 B가 검토한다. 다음 원칙은 확정한다.

- malformed XML/JSON은 해당 후보의 recoverable issue다.
- 하나의 detector 오류가 전체 scan을 중단하지 않는다.
- 파일 내용은 parser/detector가 읽는다.
- 하나의 marker에서 여러 결과가 나올 가능성을 막지 않는다.

## 6. 오류 계약

### 6.1 확정된 분류 원칙

Recoverable issue 예시:

- 접근 권한 오류
- 손상된 project manifest
- 해석하지 못한 MSBuild 변수
- 존재하지 않는 Workspace 참조
- 개별 adapter 실패

Terminal error 예시:

- config 자체가 유효하지 않음
- DB를 열거나 migration할 수 없음
- repository 저장 실패
- context 취소
- 결과 구조가 손상되어 신뢰할 수 없음

결과 상태:

| 조건 | 상태 |
|---|---|
| 오류 없음 | `COMPLETED` |
| recoverable issue가 하나 이상 존재 | `COMPLETED_WITH_ERRORS` |
| terminal error 발생 | `FAILED` |

### 6.2 현재 구현과 후속 계약

현재 `scanner.Issue`는 Go error wrapping에 적합하다.

```go
type Issue struct {
    Path      string
    Operation string
    Err       error
}
```

하지만 CLI·JSON·DB를 통과할 공용 issue는 구조화된 code와 stage가 필요하다.

```go
type ScanIssue struct {
    Path        string `json:"path"`
    Stage       string `json:"stage"`
    Code        string `json:"code"`
    Message     string `json:"message"`
    Recoverable bool   `json:"recoverable"`
}
```

`Stage`, `Code`, JSON schema는 공용 계약이므로 합의 전에는 기존 `scanner.Issue`를 변경하지 않는다.

CLI 종료 코드 제안:

| 상황 | 제안 코드 |
|---|---:|
| 정상 또는 recoverable issue와 함께 완료 | `0` |
| config·DB·repository 등 terminal error | `1` |
| 사용자 취소 | `130` |

recoverable issue를 코드 `0`으로 처리할지 별도 코드를 사용할지는 C가 자동화 사용 사례와 함께 확정해야 한다.

## 7. DB repository 계약

### 7.1 현재 상태

- `ScanRepository`는 scan 실행 요약을 저장한다.
- migration에는 `projects`, `resources`, `dependencies`, `evidence`가 존재한다.
- domain 모델과 모든 DB column을 연결하는 repository는 아직 없다.
- `projects.normalized_path`는 `UNIQUE`지만 domain에는 해당 필드가 없다.

### 7.2 제안 계약

단건 upsert:

```go
type ProjectRepository interface {
    Upsert(ctx context.Context, project domain.BuildProject, scanID string) error
}
```

scan 단위 원자적 교체:

```go
type ProjectRepository interface {
    ReplaceScanProjects(
        ctx context.Context,
        scanID string,
        projects []domain.BuildProject,
    ) error
}
```

권장 방향은 `ReplaceScanProjects`다. 부분 성공 결과는 저장하되 한 번의 DB transaction 안에서 현재 scan 결과를 반영할 수 있기 때문이다.

다음 사항을 합의한 뒤 구현한다.

1. BuildProject 필드와 DB column mapping
2. stable ID 생성 규칙
3. 기존 project를 ACTIVE로 갱신하는 조건
4. 이번 scan에서 보이지 않은 project를 STALE로 바꾸는 시점
5. partial parse 결과를 project로 저장할지 issue만 저장할지
6. Workspace 및 WorkspaceProject 저장 방식

## 8. 결과 모델 계층 계약

서로 다른 목적의 결과를 한 타입에 합치지 않는다.

```text
scanner.Result
└─ A의 파일시스템 탐색 통계

app.ScanResult
└─ A+B 분석과 DB 반영을 합친 application 결과

output.ScanView
└─ C가 text/JSON으로 출력하는 화면 모델
```

제안하는 application 결과:

```go
type ScanResult struct {
    ScanID              string
    RootsScanned        int
    FilesInspected      int64
    Directories         int64
    ProjectsDiscovered  int
    ResourcesDiscovered int
    DependenciesFound   int
    LogicalSize         int64
    Issues              []ScanIssue
    Status              string
}
```

필드명과 JSON tag는 CLI 공개 계약이 되므로 A·B·C 합의 후 추가한다.

## 9. 진행률 계약

C가 scanner 내부 상태를 직접 읽지 않는다. application service가 callback으로 진행률을 전달한다.

```go
type ProgressEvent struct {
    Phase       ProgressPhase
    CurrentPath string
    Files       int64
    Projects    int64
    Resources   int64
}
```

제안 phase:

```text
SCANNING_FILES
DETECTING_PROJECTS
DETECTING_RESOURCES
ANALYZING_DEPENDENCIES
SAVING_RESULTS
COMPLETED
```

전달 빈도 제안:

- phase가 바뀔 때 즉시 전달
- 같은 phase에서는 100개 파일 또는 200ms 중 먼저 도달한 조건으로 제한
- JSON 모드에서는 진행률을 stderr로 보내거나 비활성화

throttling 위치와 callback 오류 처리 규칙을 합의한 뒤 구현한다.

## 10. CLI 및 JSON 계약

제안 명령:

```text
libra scan
libra scan --root D:\Projects
libra scan --root C:\Source --root D:\Projects
libra scan --full
libra scan --json
```

제안 의미:

- `--root`는 반복 가능하다.
- `--root`가 하나라도 있으면 config의 project roots를 대체한다.
- `--full`이 없으면 변경된 경로만 증분 scan한다. 증분 구현 전에는 명시적인 미지원 동작이 필요하다.
- stdout에는 최종 text 또는 JSON 결과만 쓴다.
- stderr에는 진행률과 경고만 쓴다.
- JSON stdout 앞뒤에 일반 문자열을 섞지 않는다.

현재 `cmd/scan.go`는 단일 string `--root`와 placeholder 실행을 사용한다. 이 계약은 C 소유 영역이며 공개 CLI 변경이므로 이 문서만으로 즉시 수정하지 않는다.

## 11. 팀별 다음 결정

### A: Indexing & Platform

- DB column과 BuildProject 필드 mapping 제안
- stable project ID 방식 제안
- `ReplaceScanProjects` transaction 경계 제안
- STALE 전환 시점 제안
- structured issue 저장 방식 제안

### B: Dependency Analysis

- Workspace와 BuildProject 탐지 규칙 확정
- 공용 detector interface 채택 여부
- WorkspaceProject relation 저장 방식 제안
- parser 오류 code와 stage 목록 제안
- project root와 marker path의 구분 확정

### C: CLI & Output

- `app.ScanResult` 필드 및 JSON tag 확정
- `ProgressEvent` phase와 throttling 확정
- `--root` 반복·대체 규칙 확정
- recoverable issue 종료 코드 확정
- stdout/stderr 분리 테스트 추가

## 12. 적용 순서

1. 이 문서를 A·B·C가 검토한다.
2. project identity와 path 필드를 확정한다.
3. 필요한 domain 변경을 별도 PR로 만든다.
4. DB migration과 ProjectRepository를 별도 PR로 만든다.
5. detector orchestration과 structured issue를 구현한다.
6. `app.ScanResult`와 progress callback을 구현한다.
7. C가 CLI·JSON에 연결한다.
8. Windows/macOS fixture 통합 테스트를 추가한다.

각 단계는 빌드 가능한 작은 PR로 나누며 공용 schema/domain/CLI 변경은 최소 2명의 승인을 받는다.

## 13. 통합 완료 체크리스트

```text
[ ] Workspace와 BuildProject 수가 합의한 규칙대로 계산된다.
[ ] 동일한 Windows 경로 표기가 하나의 identity로 합쳐진다.
[ ] A는 scanner.Entry만 전달하고 프로젝트 의미를 해석하지 않는다.
[ ] parser 오류가 다른 후보와 전체 scan을 중단하지 않는다.
[ ] 부분 성공 결과와 issue가 함께 DB에 저장된다.
[ ] 현재 scan에 없는 project의 상태 전환 규칙이 적용된다.
[ ] progress phase가 공용 enum만 사용한다.
[ ] JSON stdout에 진행률이나 로그가 섞이지 않는다.
[ ] gofmt, go test ./..., go vet ./..., go build ./...가 통과한다.
[ ] Windows와 macOS CI가 통과한다.
```
