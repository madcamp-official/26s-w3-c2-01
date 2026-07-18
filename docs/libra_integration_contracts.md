# Libra 통합 계약 및 사전 합의

> 상태: 초안 v0.4 (Day 4 구현 현황 동기화 + Node workspace 재결정)
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

현재 Git detector와 MSBuild BuildProject·Workspace parser는 모두 외부 입력으로
`scanner.Entry`를 받는다. parser 내부에서 파일 내용이 필요할 때만
`entry.Path`를 사용하며, 수정 시각은 scanner가 수집한 `entry.ModifiedAt`을
재사용한다. Resource detector는 머신 환경을 탐지하므로 entry 기반 detector와
별도 interface를 유지한다. 다음 원칙은 확정한다.

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

### 7.0 로컬 파일 위치 (`CONFIRMED`)

`libra init`이 만드는 두 파일의 이름과 위치는 PR #13에서 구현 중 임의로
정했던 값을 그대로 확정한다.

```text
.libra.yaml   -- --config로 지정한 경로, 없으면 현재 디렉터리
.libra.db     -- .libra.yaml과 같은 디렉터리
```

`cmd/db.go`의 `configFilePath()`/`dbFilePath()`가 이 규칙을 구현한다. 다른
이름이나 위치가 필요해지면 이 항목을 다시 `DECISION_REQUIRED`로 되돌리고
갱신한다.

### 7.1 현재 상태

- `ScanRepository`는 scan 실행 요약을 저장한다.
- migration에는 `projects`, `resources`, `dependencies`, `evidence`가 존재한다.
- `ResourceRepository`는 Resource 단건 upsert, ID 조회, type 조회를 제공한다.
- `DependencyRepository`는 Dependency와 Evidence를 하나의 transaction으로
  upsert하고 프로젝트→리소스 및 리소스→프로젝트 조회를 제공한다.
- Evidence는 `scan_id`로 수집 scan에 귀속되며 기존 row도 legacy scan으로
  보존된다.
- dependency 양방향 조회는 전용 covering index를 사용하며 query-plan 테스트와
  10,000 edge benchmark가 존재한다.
- `ProjectRepository`는 관측 성공 project batch upsert와 ID·manifest 조회를
  제공하며 미발견 project 상태는 변경하지 않는다.
- `WorkspaceRepository`는 workspace upsert와 membership 원자 교체를 제공한다.
- `projects`는 root와 manifest 표시·정규화 경로를 분리하고
  `(project_type, normalized_manifest_path)`를 identity unique key로 사용한다.

### 7.2 Project·Workspace repository (`CONFIRMED`, snapshot 전환은 `PLANNED`)

현재 단계에서는 관측에 성공한 project만 batch upsert하고 자동 `STALE` 전환은
하지 않는다. scan coverage와 unverified scope 없이 전체 교체하면 접근 실패한
project를 잘못 `STALE`로 만들 수 있기 때문이다.

```go
type ProjectRepository interface {
    UpsertObserved(context.Context, string, []domain.BuildProject) error
    FindByID(context.Context, string) (domain.BuildProject, error)
    FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error)
}

type WorkspaceRepository interface {
    Upsert(context.Context, string, domain.Workspace) error
    ReplaceMembers(context.Context, string, []string) error
}
```

확정된 저장 규칙:

1. 정상 관측된 project는 `ACTIVE`로 upsert한다.
2. Project ID는 `project_type + normalized_manifest_path`의 stable hash다.
3. partial parse 결과는 Project row로 저장하지 않고 structured Issue로 남긴다.
4. Workspace와 membership은 별도 table/repository로 저장한다.
5. C·D 드라이브 관계는 경로 문자열이 아니라 ID로 연결한다.

추후 결정·구현할 항목:

- `FULL`·`ROOT`·`PROJECT` scan coverage
- 접근 실패 subtree를 나타내는 `UnverifiedScope`
- parser 실패한 기존 project의 `UNKNOWN` 전환
- 확인된 범위에서만 미발견 project를 `STALE`로 전환하는
  `FinalizeProjectSnapshot`
- 과거 snapshot용 `scan_projects`와 보존 기간
- `projects.last_observed_scan_id`가 참조하는 scan 삭제 정책

### 7.3 Resource repository (`CONFIRMED`)

Day 3 MVP는 현재 `resources` 테이블에 최신 관측 값을 단건 upsert하고 C가
필요한 ID·type 조회를 제공한다.

```go
type ResourceRepository interface {
    Upsert(context.Context, domain.Resource) error
    FindByID(context.Context, string) (domain.Resource, error)
    ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error)
}
```

현재 schema에는 `scan_id`와 resource status가 없으므로 이번 scan에서 보이지
않은 resource를 삭제하거나 `STALE`로 바꾸지 않는다. scan별 snapshot,
`scan_resources`, 미발견 resource의 상태 전환은 구현 전에 다시 합의한다
(`DECISION_REQUIRED`).

### 7.4 Dependency repository (`CONFIRMED`)

```go
type DependencyRepository interface {
    UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error
    FindResourcesByProject(context.Context, string) ([]domain.Dependency, error)
    FindProjectsByResource(context.Context, string) ([]domain.Dependency, error)
    FindEvidence(context.Context, string) ([]domain.Evidence, error)
}
```

첫 번째 `string`은 Evidence가 귀속될 scan ID다. Dependency와 모든 Evidence는
하나의 DB transaction에서 upsert하며 일부만 저장하지 않는다. 동일 Evidence
ID를 다시 관측하면 scan ID와 `CollectedAt`을 최신 관측으로 갱신한다.

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

- BuildProject root·manifest mapping과 stable ID 구현
- 관측 성공 project의 batch upsert 구현
- Workspace와 membership repository 구현
- structured Issue와 scan orchestration 구현
- coverage 계약 확정 뒤 `FinalizeProjectSnapshot`과 STALE 전환 구현

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
[x] A는 scanner.Entry만 전달하고 프로젝트 의미를 해석하지 않는다.
[ ] parser 오류가 다른 후보와 전체 scan을 중단하지 않는다.
[ ] 부분 성공 결과와 issue가 함께 DB에 저장된다.
[ ] 현재 scan에 없는 project의 상태 전환 규칙이 적용된다.
[ ] progress phase가 공용 enum만 사용한다.
[ ] JSON stdout에 진행률이나 로그가 섞이지 않는다.
[x] gofmt, go test ./..., go vet ./..., go build ./...가 통과한다.
[ ] Windows와 macOS CI가 통과한다.
```

---

# Part II. 후속 단계 계약

> 이 파트는 `summary → explain → impact → plan → clean → restore → daemon` 구현 전에 확정할 계약을 단계별로 정리한다. 아래 제안은 팀 합의 전까지 공개 domain, DB schema, JSON, CLI 또는 safety 동작으로 구현하지 않는다.

## 14. 후속 계약 지도

| 영역 | 결정 항목 | 결정 시점 |
|---|---|---|
| 프로젝트 모델 | root·manifest, ID, repository·workspace 관계 | 분석기 통합 전 |
| 스캔 상태 | snapshot, scope, staging·transaction | DB repository 확장 전 |
| 분석기 | 공통 결과, 실행 순서, resource 병합 | adapter orchestration 전 |
| 근거·정책 | Evidence, Confidence, Risk, Impact | explain·impact 전 |
| 사용자 계약 | config 우선순위, JSON, stderr, 종료 코드 | 실제 CLI 연결 전 |
| 테스트 | fixture, 플랫폼 책임, 성능 기준 | 통합 PR 전 |
| 계획 | 후보 선택, 실행 전 재검증 | plan 전 |
| 정리·복구 | allowlist, denylist, 격리, transaction | clean·restore 전 |
| 증분 처리 | event 병합, 유실 복구, 동시 실행 | daemon 전 |
| 협업 | 공동 소유 파일, 합의 대상, 변경 절차 | 모든 계약 변경 시 |

## 15. 프로젝트 root, manifest 및 stable ID

### 15.1 root와 manifest (`CONFIRMED`)

다음 두 값은 분리한다.

```text
RootPath:      D:\Game\Client
ManifestPath:  D:\Game\Client\Client.vcxproj
```

확정 모델:

```go
type BuildProject struct {
    ID                     string
    Name                   string
    Type                   ProjectType
    RootPath               string
    NormalizedRootPath     string
    ManifestPath           string
    NormalizedManifestPath string
    Drive                  string
    LogicalSize            int64
    LastModifiedAt         time.Time
    LastObservedAt         time.Time
    Status                 ProjectStatus
}
```

확정 의미:

- `RootPath`: 프로젝트가 소유하는 디렉터리
- `ManifestPath`: identity에 사용하는 대표 marker/config 파일
- `.vcxproj.filters`: manifest가 아닌 보조 파일
- `Directory.Build.props`, import된 `.props`·`.targets`: manifest가 아닌 Evidence source
- 독립 `.vcxproj`: marker 파일의 부모 디렉터리를 root로 사용
- Node: `package.json`을 manifest로 사용
- Git fallback: `.git` entry를 manifest로 사용
- `.sln`: BuildProject가 아니라 Workspace manifest로 사용

manifest가 여러 개인 경우와 Git repository 안에 여러 BuildProject가 있는 경우를 허용한다.

### 15.2 stable ID (`CONFIRMED`, Project·Workspace 포함)

모든 MVP identity hash는 NUL separator와 SHA-256 hex를 사용한다.

```text
Project ID    = SHA-256(project_type + NUL + normalized_manifest_path)
Workspace ID  = SHA-256(workspace_type + NUL + normalized_manifest_path)
Resource ID   = SHA-256(resource_type + NUL + version + NUL + normalized_path)
Dependency ID = SHA-256(source_type + NUL + source_id + NUL + relation + NUL + target_type + NUL + target_id)
Evidence ID   = SHA-256(dependency_id + NUL + kind + NUL + source_path + NUL + property + NUL + raw_value + NUL + resolved_value)
```

경로 기반 ID를 사용하면 이동·manifest 이름 변경은 새 객체로 취급한다. 이전
객체의 `STALE` 전환은 scan coverage 계약 구현 전까지 자동 수행하지 않는다.
직렬화 형식은 모든 플랫폼에서 같은 결과가 나오도록 golden test로 고정한다.

`.git`은 장기적으로 ProjectType이 아니라 repository metadata로 분리하는 방향을 권장한다. 사용자 대표 표시 우선순위 제안은 다음과 같다.

```text
.sln > .vcxproj/.csproj > package.json > .git
```

관계 이름 제안:

```text
WORKSPACE_CONTAINS_BUILD_PROJECT
REPOSITORY_CONTAINS_WORKSPACE
REPOSITORY_CONTAINS_BUILD_PROJECT
```

## 16. 경로 API 확장과 크기 의미

### 16.1 pathutil 후속 API (`PLANNED`)

현재 채택된 `Normalize`, `Absolute` 외에 다음 API가 필요하다.

```go
Equal(a, b string) (bool, error)
IsSameOrChild(path, parent string) (bool, error)
Volume(path string) (string, error)
Display(path string) string
```

추가로 결정하고 테스트할 경계:

- UNC 경로
- `\\?\` long path
- 드라이브 문자가 없는 network path
- 존재하지 않는 경로의 정규화
- junction·symlink의 identity와 실제 target

용도는 다음처럼 분리한다.

```text
DB 비교      → NormalizedPath
사용자 출력  → DisplayPath
파일 작업    → 실행 직전 검증한 AbsolutePath
```

### 16.2 크기 계약 (`CONFIRMED` / 확장 `PLANNED`)

MVP의 모든 크기는 `LogicalSize`이며 출력에도 “논리 크기”라고 표시한다.

```go
LogicalSize   int64
AllocatedSize *int64 // 향후 Windows 실제 할당 크기 지원 시
SizeKnown     bool
```

반드시 `0 bytes`와 `unknown`을 구분한다. hard link, sparse file, 압축 파일, 여러 프로젝트가 공유하는 산출물의 물리 크기 중복 제거는 MVP 이후 항목이다.

## 17. 현재 상태, snapshot 및 scan scope

### 17.1 현재 상태와 과거 snapshot (`DECISION_REQUIRED`)

권장 구조:

```text
projects / resources
└─ 현재 알려진 최신 상태

scan_projects / scan_resources
└─ 특정 scan에서 관찰된 snapshot

scans
└─ scan 실행과 범위
```

전체 scan에서 다시 발견되지 않은 객체를 즉시 삭제하지 않는다.

```text
LastObservedAt 유지
Status = STALE 또는 UNKNOWN
```

과거 scan과 Evidence는 감사·설명용 기록으로 유지할 수 있지만 기본 조회는 마지막으로 성공한 활성 snapshot만 사용한다.

### 17.2 scan scope (`DECISION_REQUIRED`)

```go
type ScanScope string

const (
    ScanScopeFull    ScanScope = "FULL"
    ScanScopeRoot    ScanScope = "ROOT"
    ScanScopeProject ScanScope = "PROJECT"
)
```

| Scope | 갱신 범위 | 미발견 객체 처리 |
|---|---|---|
| `FULL` | 설정된 전체 범위 | 범위 전체에서 STALE 전환 가능 |
| `ROOT` | 지정 root 내부 | root 밖은 변경 금지 |
| `PROJECT` | 지정 project와 연결 resource | 다른 project 변경 금지 |

부분 scan 결과로 다른 drive 또는 root의 객체를 STALE 처리하지 않는다. scan record에는 scope와 대상 root/project를 저장한다.

**현재 임시 동작 (`CONFIRMED`, 위 표 자체는 여전히 `DECISION_REQUIRED`):** `cmd/scan.go`는 `ScanScope`를 구현하지 않았고 `--full`은 매 실행에서 아무 효과가 없다. 이 표가 결정되기 전까지 모든 scan은 사실상 `FULL`이며, `--full` 플래그는 `cobra`의 `MarkDeprecated`로 표시해 사용할 때마다 "현재 모든 scan은 full scan" 안내를 출력한다. 플래그 자체를 지우지 않은 이유는 위 표가 확정되면 다시 의미 있는 옵션이 되기 때문이다.

### 17.3 staging과 transaction (`DECISION_REQUIRED`)

권장 흐름:

```text
1. scans = RUNNING
2. 수집 결과를 scan_id와 함께 batch 저장
3. 분석 완료
4. 하나의 commit 단계에서 활성 snapshot 전환
5. scans = COMPLETED 또는 COMPLETED_WITH_ERRORS
```

실패한 scan은 `FAILED`로 남기되 불완전한 결과를 기본 조회에 사용하지 않는다. 수집 batch와 최종 활성 snapshot 전환을 분리하고, 최종 전환만 원자적으로 처리한다.

## 18. Adapter → App → Store 계약

### 18.1 계층 책임 (`CONFIRMED`)

```text
Adapter → 사실과 분석 결과 생성
App     → 결과 병합, 실행 순서, 정책 적용
Store   → transaction과 저장
```

adapter가 DB에 직접 쓰거나 Risk를 최종 판정하지 않는다.

### 18.2 공통 분석 결과 (`CONFIRMED`)

```go
type DetectionResult[T any] struct {
    Items      []T
    Issues     []Issue
    Unverified []UnverifiedScope
}
```

역할별 interface가 공통 result envelope를 사용한다.

```go
type ProjectDetector interface {
    Observe(context.Context, scanner.Entry) DetectionResult[ProjectCandidate]
}

type ResourceDetector interface {
    Detect(context.Context, Environment) DetectionResult[domain.Resource]
}

type DependencyAnalyzer interface {
    Analyze(context.Context, domain.BuildProject, ResourceIndex) DetectionResult[DependencyBundle]
}
```

기존 adapter는 작은 wrapper로 이 계약에 연결한다. adapter별 세부 반환 타입을
application service가 type switch로 해석하지 않는다. Node project detector도
다른 project detector와 동일하게 `scanner.Entry`를 입력으로 받는다.

structured Issue의 확정 최소 필드는 다음과 같다.

```go
type Issue struct {
    Code      string
    Phase     AnalysisPhase
    Adapter   string
    Path      string
    Operation string
    Severity  IssueSeverity
    Message   string
    Cause     error // JSON과 DB 원문 저장에서는 제외
}
```

partial parse는 candidate row를 만들지 않고 Issue와 UnverifiedScope로 전달한다.

### 18.3 분석 단계 (`CONFIRMED`)

진행률 phase와 분석 orchestration에서 같은 enum을 사용한다.

```text
DISCOVER_FILES
DISCOVER_PROJECTS
DISCOVER_SYSTEM_RESOURCES
ANALYZE_PROJECT_SETTINGS
RESOLVE_DEPENDENCIES
CLASSIFY_ARTIFACTS
CALCULATE_RISK
PERSIST_RESULTS
COMPLETED
```

기존 문서의 `SCANNING_FILES` 등 phase 이름은 위 목록과 통합해야 하며 두 종류의 문자열을 동시에 유지하지 않는다.

현재 구현 범위에서는 위 enum과 순서를 고정하되 progress throttling과 callback
주기는 CLI 연결 전에 C와 결정한다. scan orchestration의 최종 결과는
`scanner.Result`와 구분되는 `app.ScanResult`로 반환한다.

### 18.4 Resource 병합 (`CONFIRMED`)

```text
ResourceKey = Type + Version + NormalizedPath
```

Resource의 사용자 표시 경로와 비교용 경로는 별도 필드로 유지한다. 기존
`Path` 필드는 `DisplayPath`로 이름을 변경한다.

```go
type Resource struct {
    ID              string
    Name            string
    Type            ResourceType
    Version         string
    DisplayPath     string
    NormalizedPath  string
    LogicalSize     int64
    SizeKnown       bool
    ReclaimableSize int64
    Regenerable     bool
    SystemManaged   bool
    LastModifiedAt  *time.Time
    LastObservedAt  time.Time
    Risk            RiskLevel
    Confidence      int
}
```

필드 충돌 시 근거 우선순위:

```text
공식 명령 결과 > 설치 metadata > 디렉터리 추론
```

병합 과정에서도 각 source Evidence는 버리지 않는다.

### 18.5 Dependency graph (`CONFIRMED`)

Day 4 MVP는 typed endpoint를 가진 `PROJECT -> RESOURCE` 방향의
`REQUIRES` edge를 저장한다.

```go
type Dependency struct {
    ID         string
    SourceType NodeType
    SourceID   string
    TargetType NodeType
    TargetID   string
    Relation   RelationType
    Confidence int
}
```

드라이브 문자는 edge 자체에 저장하지 않는다. C·D 드라이브 간 관계도
각 endpoint ID로 연결하며 프로젝트와 리소스의 표시 경로는 각 repository에서
조회한다.

## 19. 분석기별 경계

### 19.1 MSBuild (`DECISION_REQUIRED`)

MVP Evidence mapping:

```text
직접 literal 값           → DECLARED
단순 property 치환 성공   → RESOLVED
조건부 속성 중 하나       → INFERRED
해석하지 못한 변수        → UNKNOWN
MSBuild preprocess 결과   → RESOLVED
```

권장 requirement:

```go
type Requirement struct {
    RawValue      string
    ResolvedValue string
    Configuration string
    Platform      string
}
```

Configuration·Platform을 분석하지 않았으면 반드시 `UnverifiedScope`를 남긴다. `Latest`, `10.0`, 미설치 SDK, Debug/Release 차이 및 조건별 dependency 표현은 B가 구현 전에 확정한다.

### 19.2 Node workspace (`CONFIRMED`, MVP 범위 — 2026-07-18 재결정)

```text
각 package.json  → BuildProject 후보
workspace root   → Workspace
root node_modules → workspace 소유 Resource
하위 node_modules → 해당 package 소유 Resource
```

Node adapter 구현 전에 결정하기로 했던 6개 항목을 `internal/adapter/node`
(Mac C 소유 영역)에서 MVP 범위로 확정하고 구현했다. 처음에는 workspace
지원 자체를 범위 밖으로 미뤘었는데("관계/공용 자원 연결을 지원해야 하지
않냐"는 지적으로) 이번에 다시 열어서 아래처럼 재결정했다.

| 항목 | MVP 결론 |
|---|---|
| npm/pnpm/Yarn workspace 지원 범위 | **지원(재결정).** `package.json`의 `workspaces` 필드(npm/Yarn, 배열 또는 `{packages:[...]}` 객체 형태 모두 인정)와 `pnpm-workspace.yaml`(pnpm)을 읽어 workspace root를 탐지한다(`DetectWorkspace`). member는 `filepath.Glob` 기반 단일 세그먼트 glob만 지원한다 — `packages/*`는 되지만 재귀 `**`는 세그먼트 하나로만 매칭되고, `!` 부정 패턴은 적용되지 않고 그냥 건너뛴다(제외 안 됨, 안전한 쪽으로 미지원). 중첩 workspace(member가 또 다른 workspace root인 경우)는 한 단계만 풀고 재귀하지 않는다. |
| 여러 lockfile의 우선순위 | 불필요. `package-lock.json`/`npm-shrinkwrap.json`/`pnpm-lock.yaml`/`yarn.lock` 중 하나라도 있으면 재생성 근거로 충분하다고 본다(존재 여부만 확인, 어떤 패키지 매니저인지는 판단하지 않음). workspace member는 자기 디렉터리뿐 아니라 workspace root의 lockfile도 근거로 인정한다(`DetectMemberArtifacts`) — 실제로 npm/Yarn/pnpm workspace는 보통 lockfile을 root에 하나만 둔다. |
| lockfile 없는 node_modules의 재생성 가능성 | `Regenerable=false`. `Confidence`도 낮춰서(§20.2 확정 전 임시값) INFERRED 수준으로 취급한다. |
| malformed package.json 저장 방식 | `Detector.Detect`가 error를 반환한다. 다른 후보나 전체 scan을 막지 않는 recoverable 실패로 간주하되(§5), orchestration이 아직 없어 실제 issue 수집·저장은 후속 작업이다. |
| nested node_modules 탐색 | **부분 지원(재결정).** project root 바로 아래는 그대로 보고, workspace member는 `ResolveMembers`로 찾은 각 member 디렉터리 바로 아래만 추가로 본다 — member 안에 또 nested node_modules(예: 3단계 이상 깊이)가 있으면 여전히 안 본다. root의 `node_modules`는 한 번만 Resource로 만들고, 자기 것이 없는 member는 root 것을 공유(`SharesRootNodeModules=true`)한다고만 표시한다 — 같은 디렉터리를 member 수만큼 중복 Resource로 만들지 않는다(§3.1 "디렉터리 논리 크기는 한 번만 계산"). |
| `.pnpm` store 크기 소유권 | 범위 밖. 전역 pnpm store 분석은 원래 일정에서도 P1(`pnpm 전역 저장소`)이라 이번 결정에 포함하지 않는다. |

**중요한 제약**: member가 workspace root의 공유 자원을 쓴다는 관계
(`MemberArtifacts.SharesRootNodeModules`)는 `domain.Dependency`
그래프(§18.5, 이제 `CONFIRMED`)에 아직 저장하지 않는다. `PROJECT ->
RESOURCE REQUIRES` edge를 만들려면 안정적인 `BuildProject` ID가 필요한데
그건 여전히 `DECISION_REQUIRED`다(§7.2, §15.2, Windows A 담당). 그래서 이
관계는 지금 `internal/adapter/node` 안에서만 값으로 존재하고, Project ID가
정해지면 바로 Dependency edge로 옮겨 담을 수 있게 모양만 맞춰뒀다.

### 19.3 산출물 판정 (`DECISION_REQUIRED`)

이름이 `build`, `bin`, `out`이라는 이유만으로 SAFE가 될 수 없다. SAFE 후보 최소 조건:

```text
1. 발견된 project root 내부
2. 알려진 이름 또는 설정의 output path
3. reparse point 아님
4. Git tracked 원본 파일 없음
5. 재생성 Evidence 존재
```

이름만 일치하면 `build-output + REVIEW + INFERRED`, 설정에서 확인되면 `SAFE 후보 + DECLARED/RESOLVED`로 처리한다.

이 절 자체는 adapter 전반에 걸친 결정이라 Node adapter 하나로 `CONFIRMED`
처리하지 않는다. 다만 `internal/adapter/node`의 현재 구현은 이미 이 원칙을
따른다: `dist`/`.next`/`build`/`out`은 디렉터리 이름만으로 판정하므로 항상
`INFERRED` 수준 Confidence를 부여하고, Git tracked 원본 확인이나 output
path 설정 파싱은 하지 않는다(2·3·4번 조건 미검증). `RiskPolicy`에 SAFE
분기가 아직 없어(§20.3 코드 참고) 실제 저장되는 Risk는 현재 모두
`REVIEW`로 귀결된다.

## 20. Evidence, Confidence, Risk 및 Impact

### 20.1 Evidence (`CONFIRMED`, 만료·redaction은 `DECISION_REQUIRED`)

권장 필드:

```go
type Evidence struct {
    ID            string
    DependencyID  string
    Kind          EvidenceKind
    SourcePath    string
    Property      string
    RawValue      string
    ResolvedValue string
    CollectedAt   time.Time
}
```

계약 제안:

- Evidence는 `evidence.scan_id` foreign key로 특정 scan에 귀속한다 (`CONFIRMED`).
- 현재 repository는 dependency에 연결된 Evidence를 모두 반환한다. 최신 유효
  Evidence 필터링은 만료 정책을 확정한 뒤 application 계층에 추가한다.
- 과거 Evidence는 기록으로 유지할 수 있다.
- `RawValue` 저장은 지원하지만 민감 값 redaction 정책은 아직 없다. 정책 확정
  전에는 adapter가 비밀·개인정보 원문을 전달하지 않는다.
- source 파일이 바뀌었을 때 기존 Evidence의 유효성을 다시 평가하는 기능은
  아직 구현하지 않았다.

내용 기반 Evidence ID를 중복 키로 사용하고 같은 근거를 다시 발견하면
`CollectedAt`을 갱신한다 (`CONFIRMED`). 만료 정책과 raw value redaction은
구현 전에 확정한다.

기존 Evidence는 migration이 생성하는 `migration:003:legacy-evidence` scan에
귀속하여 삭제 없이 보존한다. Dependency는 최신 관계를 upsert하며 과거 graph
snapshot과 미발견 관계 삭제는 snapshot 계약을 확정할 때까지 수행하지 않는다.

### 20.2 Confidence (`DECISION_REQUIRED`)

MVP 기본 점수 제안:

```text
RESOLVED  90
OBSERVED  85
DECLARED  75
INFERRED  40
UNKNOWN   10
```

복수 Evidence를 단순 합산하지 않는다.

```text
기본 점수       = 가장 강한 Evidence
서로 다른 보조 근거 = 제한된 가산
UnverifiedScope = 항목별 감점
최종 범위       = 0..100
```

Confidence가 높다는 사실은 Risk가 SAFE라는 의미가 아니다.

### 20.3 Risk 중앙 정책 (`CONFIRMED`)

adapter는 사실과 Evidence만 반환하고 application의 `RiskPolicy`가 판정한다.

```go
type RiskPolicy interface {
    Classify(ResourceContext) RiskAssessment
}
```

MVP 결정표:

| 조건 | Risk |
|---|---|
| 보호 경로 내부 | `BLOCKED` |
| 현재 project가 요구하는 SDK | `BLOCKED` |
| 사용자 데이터 가능성 | `BLOCKED` |
| Git tracked 원본 포함 | `BLOCKED` |
| 재생성 가능하지만 Evidence가 약함 | `REVIEW` |
| project 내부 산출물이고 재생성 Evidence가 명확함 | `SAFE` |
| 분석 실패·불명확 | 최소 `REVIEW` |
| 경로가 사라짐·변경됨 | 실행 대상 제외 |

Windows MVP 보호 경로는 현재 장비에서 확인되는 `%WINDIR%`,
`%ProgramFiles%`, `%ProgramFiles(x86)%`, `%ProgramData%`로 확정한다. A의
경로 분류기는 해당 경로 내부 여부와 근거를 반환하고, 중앙 `RiskPolicy`가
`SystemManaged=true`, `Risk=BLOCKED`를 적용한다. Adapter는 Risk를 직접
판정하지 않는다.

### 20.4 Impact (`DECISION_REQUIRED`)

```go
type ImpactScope string // RUN, BUILD, DEBUG, RESTORE, CI
type ImpactLevel string // NONE, LOW, HIGH, UNKNOWN
```

`likely unaffected`, `expected to fail` 같은 문장은 domain 값이 아니라 output formatter가 enum을 변환해 만든다.

## 21. CLI와 JSON 확장 계약

### 21.1 config와 CLI 우선순위 (`DECISION_REQUIRED`)

```text
CLI option > 명시한 config > 기본 config > 코드 default
```

- `--root`가 하나라도 있으면 config roots 대체
- `--exclude`는 config exclude에 추가

### 21.2 공통 JSON envelope (`DECISION_REQUIRED`)

```json
{
  "schema_version": 1,
  "command": "scan",
  "status": "completed_with_errors",
  "data": {},
  "warnings": [],
  "generated_at": "2026-07-18T07:00:00Z"
}
```

공통 규칙 제안:

- bytes: `int64`
- time: RFC3339 UTC
- enum: domain의 고정 문자열
- 사람이 읽는 단위 변환: text formatter에서만 수행
- 배열 정렬 기준을 command별로 고정
- field 생략과 `null` 의미를 schema에 명시

### 21.3 stdout·stderr와 종료 코드 (`DECISION_REQUIRED`)

```text
stdout → 최종 결과
stderr → progress, warning, verbose log
```

JSON stdout에는 다른 문자를 절대 섞지 않는다. 비TTY에서는 progress를 자동 비활성화하는 방향을 권장한다.

| 코드 | 의미 |
|---:|---|
| `0` | 정상 또는 recoverable warning과 함께 완료 |
| `1` | 일반 실행 실패 |
| `2` | 잘못된 option·config |
| `3` | 대상 없음 |
| `4` | safety policy 차단 |
| `5` | 부분 clean·restore 실패 |
| `130` | 사용자 취소 |

scan의 개별 접근 오류는 `0`, DB 저장·pipeline 실패는 `1`로 처리하는 방향을 제안한다.

### 21.4 대상 식별 (`DECISION_REQUIRED`)

```text
명시적 prefix → 지정 type
절대 경로     → path 검색
그 외         → ID 또는 이름 검색
```

동명 대상이 여러 개면 자동 선택하지 않고 정확한 ID 또는 경로를 요구한다.

## 22. Fixture, 플랫폼 및 성능

### 22.1 공용 fixture (`PLANNED`)

```text
testdata/
├─ filesystem/
├─ solutions/
├─ msbuild/
├─ dotnet/
├─ node/
├─ safety/
└─ golden/
```

각 fixture는 project/resource/dependency/issue 기대 결과를 machine-readable 파일로 포함한다. A·B·C가 같은 fixture를 사용한다.

필수 사례:

- 경로: 대소문자, 상대 경로, 중복·없는 root, UNC, long path, junction, symlink, 접근 거부
- project: solution+복수 project, 독립 project, broken solution, 없는 참조, shared project, Git 내부 복수 project
- MSBuild: 직접·빈·property SDK, Directory.Build.props, 조건부 group, Debug/Release 차이, custom OutputPath
- Node: manifest만 존재, lockfile, node_modules, monorepo, malformed manifest, 복수 lockfile
- DB: 재실행, 부분 scan, 중간 실패, migration 재실행, dependency 중복, STALE 전환

### 22.2 플랫폼 책임 (`CONFIRMED`)

```text
macOS/비Windows:
domain, output, config, parser, repository

Windows:
path/drive/reparse point, vswhere, Windows SDK, 실제 MSBuild, clean/restore
```

Windows 전용 기능은 비Windows에서 panic·빌드 실패 대신 명확한 unsupported 결과를 반환한다.

Windows SDK, Windows용 .NET SDK, Visual Studio 탐지기는 공통
`adapter.ErrUnsupportedPlatform`을 반환하여 "설치 없음"과 "현재 플랫폼에서
지원하지 않음"을 구분한다.

### 22.3 성능 기준 (`PLANNED`)

```text
100,000 파일에서 memory 300MB 이하
모든 file path를 한꺼번에 보관하지 않음
DB insert 100~1,000개 batch
취소 후 빠르게 종료
```

scanner visitor는 후보 수집·가벼운 분류만 수행하고 XML/JSON parser와 DB writer는 별도 bounded worker/batch로 운영한다. 정확한 시간보다 memory 상한과 무한 대기 방지를 우선한다.

## 23. Plan 계약

### 23.1 후보 선택 (`DECISION_REQUIRED`)

MVP greedy 순서:

```text
1. BLOCKED 제외
2. 요청한 Risk 범위만 유지
3. Risk가 낮은 순
4. Confidence가 높은 순
5. 큰 Resource 순
6. target bytes 이상이면 중단
```

최적 부분집합은 풀지 않는다. 실행 단위는 Resource이며 text 출력에서 project별로 묶는다.

### 23.2 계획 이후 변경 재검증 (`DECISION_REQUIRED`)

```go
type PlanItem struct {
    ResourceID       string
    Path             string
    ExpectedSize     int64
    ExpectedModified time.Time
    ExpectedType     ResourceType
    RiskAtPlanning   RiskLevel
}
```

실행 직전에 path, type, size, modified time, Risk와 safety 조건을 다시 검사한다. 불일치하면 항목을 차단하고 새 plan 생성을 요구한다.

## 24. Clean과 Restore 안전 계약

### 24.1 allowlist (`DECISION_REQUIRED`, safety review 필수)

MVP 후보 이름:

```text
node_modules, bin, obj, build, dist, .next, out, Debug, Release
```

이름만으로 허용하지 않는다. 모든 조건이 필요하다.

```text
project root 내부
Risk == SAFE
Regenerable == true
reparse point 아님
보호 경로 아님
실행 직전 재검증 성공
```

### 24.2 denylist (`DECISION_REQUIRED`, safety review 필수)

```text
C:\Windows
C:\Program Files
C:\Program Files (x86)
사용자 문서
.git과 .git\objects
.env, 인증서, key
DB 파일
quarantine 자체
알 수 없는 대용량 경로
```

denylist path와 그 하위 path 모두 차단한다.

### 24.3 link 정책 (`DECISION_REQUIRED`)

MVP에서는 `FollowReparsePoints` 설정과 관계없이 다음으로 고정하는 방향을 권장한다.

```text
scan    → link 자체만 기록, target 미추적
clean   → symlink/junction/reparse point BLOCKED
restore → manifest에 기록된 일반 directory만 처리
```

현재 실질적으로 지원하지 않는 `FollowReparsePoints=true`는 제거하거나 명시적인 unsupported 오류를 반환해야 한다.

### 24.4 quarantine (`DECISION_REQUIRED`)

동일 volume의 전용 디렉터리를 사용한다.

```text
D:\.libra-quarantine\tx-...\
├─ manifest.json
└─ items\
```

volume별 위치, hidden attribute, ACL 상속, 공간 부족, 기존 quarantine 충돌을 구현 전에 확정한다.

### 24.5 transaction과 부분 실패 (`DECISION_REQUIRED`)

transaction 상태 제안:

```text
PLANNED, RUNNING, QUARANTINED, PARTIALLY_QUARANTINED,
RESTORED, PARTIALLY_RESTORED, PURGED, FAILED
```

item 상태 제안:

```text
PENDING, MOVED, SKIPPED, FAILED, RESTORED
```

MVP는 item을 독립 처리하고 실패해도 나머지를 계속하며 최종 상태를 `PARTIALLY_QUARANTINED`로 기록한다. 파일 이동 전에 disk manifest를 쓰고 각 이동 후 갱신해 DB 기록 실패에서도 복구 근거를 남긴다.

### 24.6 restore 충돌과 purge (`DECISION_REQUIRED`)

- 원래 위치가 존재하면 자동 overwrite·merge 금지
- 해당 item 복구 차단
- 향후 `--restore-to` 고려
- `quarantine_days`는 자동 삭제가 아니라 purge 가능 표시 기준
- 자동 영구 삭제 없음
- purge는 별도 명시적 명령과 dry-run 필요

## 25. Incremental Scan과 Daemon

### 25.1 event 병합 (`PLANNED`)

```text
path별 500ms~2s debounce
중복 CREATE/WRITE/RENAME 병합
project root 단위 재분석
```

file 하나의 dependency만 직접 고치지 않고 project 단위로 안전하게 재분석한다.

### 25.2 event 유실 복구 (`PLANNED`)

```text
daemon event   → 빠른 갱신
주기적 full scan → 정확성 복구
```

파일 변경을 관찰한 것과 SDK 실제 사용을 관찰한 것은 서로 다른 Evidence다.

### 25.3 동시 실행 (`DECISION_REQUIRED`)

```text
DB writer는 한 번에 하나
read command는 허용
full scan 중 새 full scan 차단
daemon은 full scan 중 event queue 유지
```

application lock 또는 DB lock 방식을 구현 전에 결정한다.

## 26. 공동 소유와 계약 변경 절차

### 26.1 공동 소유 파일

```text
internal/domain/*
internal/app/*
internal/store/sqlite/migrations/*
internal/output의 JSON view
cmd의 공통 option
docs의 command 계약
testdata의 기대 결과
```

### 26.2 반드시 팀 합의가 필요한 변경

```text
enum 추가·이름 변경
domain field 추가·삭제
DB migration
JSON field 변경
CLI option 이름·default 변경
Risk·Confidence 공식 변경
project 중복 기준 변경
clean allowlist·denylist 변경
exit code 변경
```

외부 동작이 바뀌지 않는 private refactor, 성능 최적화, test 추가, 작은 내부 오류 문구 개선은 담당자 판단으로 진행할 수 있다.

### 26.3 계약 변경 절차

```text
1. PR에 "Contract change" 표시
2. 변경 전후 예시 작성
3. domain + migration + output + docs 동시 변경
4. fixture와 golden 결과 수정
5. 다른 팀원 최소 1명 승인
6. safety 변경은 나머지 두 명 모두 승인
```

## 27. 단계별 결정 체크리스트

### 지금 결정

```text
[x] Project root와 manifest 의미
[x] Project ID 규칙
[x] Dependency·Evidence ID 규칙
[x] Resource ID 규칙
[x] 공용 경로 정규화
[ ] FULL·ROOT·PROJECT scan 의미
[ ] 현재 상태와 snapshot 저장
[x] Adapter 공통 반환 타입
[x] Project repository interface
[x] Resource repository interface
[x] Dependency repository interface
[x] structured Issue
[ ] JSON envelope와 exit code
```

### 분석기 연결 전

```text
[ ] MSBuild 해석 수준
[x] Node monorepo 경계 (단일 세그먼트 glob member 해석 + 공유 node_modules 표시까지 지원, 중첩 workspace·재귀 glob·`.pnpm` store는 범위 밖, §19.2)
[x] Resource 병합 규칙
[x] Evidence field
[ ] Evidence 만료와 redaction
[ ] Confidence 공식
[x] 중앙 RiskPolicy
[ ] Impact enum
[ ] 산출물 소유권 판정
```

### 정리 기능 전

```text
[ ] SAFE allowlist
[ ] 절대 금지 denylist
[ ] link/reparse point 정책
[ ] plan 실행 전 재검증
[ ] quarantine와 manifest
[ ] transaction/item 상태
[ ] 부분 실패 정책
[ ] restore 충돌
[ ] purge 정책
```

### daemon·배포 전

```text
[ ] event 병합과 유실 복구
[ ] 동시 실행 lock
[ ] export schema version
[ ] Windows CI 필수 범위
[ ] 성능 기준
[ ] log 개인정보 redaction
[ ] schema 호환성과 migration 정책
```

## 28. 우선순위

Project identity와 orchestration 기본 계약은 확정되었다. 후속 구현 전에 남은
계약을 다음 순서로 고정한다.

```text
1. scan scope, UnverifiedScope와 DB snapshot 갱신
2. Evidence 만료·redaction과 Confidence 공식
3. Impact enum과 산출물 소유권
4. JSON envelope, stderr, exit code
5. cleanup·restore safety 계약
6. daemon 동시 실행과 event 유실 복구
```

이 계약을 먼저 고정하면 A·B·C가 scanner/store, Windows adapter, CLI/Node를 독립적으로 구현하면서도 후반의 대규모 domain·schema·output 재작성을 피할 수 있다.

## 29. Project index 및 orchestration 구현 추적

### 이번 구현에서 완료할 항목

```text
[x] Node detector scanner.Entry 입력 통일
[x] BuildProject·Workspace path model과 stable ID
[x] projects schema migration과 ProjectRepository
[x] workspaces·workspace_projects migration과 WorkspaceRepository
[x] structured Issue·DetectionResult·AnalysisPhase
[x] scan orchestration과 app.ScanResult
[x] Windows test, macOS cross compile, golden ID test
```

### 추후 합의 후 구현할 항목

```text
[ ] FULL·ROOT·PROJECT scope의 정확한 coverage
[ ] 접근 실패 subtree와 candidate를 표현하는 UnverifiedScope 세부 schema
[ ] ProjectStatus UNKNOWN·STALE 전환 조건
[ ] FinalizeProjectSnapshot transaction과 활성 snapshot 전환
[ ] scan_projects·scan_resources 과거 snapshot
[ ] scan record 삭제와 last_observed_scan_id foreign key 정책
[ ] progress callback throttling과 stderr 표현
[ ] structured Issue DB 보존 기간과 Cause redaction
[ ] ResourceService의 enrich와 persist 단계 완전 분리
```

위 추후 항목이 확정되기 전에는 미발견 project를 자동으로 `STALE` 처리하거나
부분 scan 결과로 다른 root·drive의 상태를 변경하지 않는다.
