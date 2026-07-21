# libra CLI 명령어 및 개발 일정

> 로컬 개발 프로젝트와 SDK·도구·캐시·빌드 산출물 사이의 의존 관계를 분석하고, 삭제 영향을 설명하며 안전한 저장공간 정리 계획을 제공하는 Windows 우선 CLI 도구

---

## 1. 프로젝트 기본 정보

### 1.1 기술 스택

- **언어:** Go
- **CLI 프레임워크:** Cobra
- **데이터베이스:** SQLite
- **설정 파일:** YAML
- **우선 지원 OS:** Windows 10/11
- **개발 환경:** Windows 2명, macOS 1명
- **배포 형태:** 단일 실행 파일
- **실시간 모니터링:** 핵심 기능 완료 후 선택 구현

### 1.2 핵심 사용자 질문

libra는 다음 질문에 근거와 함께 답하는 것을 목표로 한다.

1. 이 파일이나 SDK는 왜 내 컴퓨터에 존재하는가?
2. 어떤 프로젝트가 이 리소스를 사용하고 있는가?
3. C 드라이브의 이 리소스를 지우면 D 드라이브 프로젝트가 영향을 받는가?
4. 이미 빌드된 프로그램 실행과 프로젝트 재빌드에 각각 어떤 영향이 있는가?
5. 안전하게 목표 용량만큼 공간을 확보하려면 무엇을 정리해야 하는가?

### 1.3 MVP 원칙

- 기본 동작은 **읽기 전용 분석**이다.
- 모든 정리 명령은 기본적으로 `dry-run`을 지원한다.
- Windows SDK, Visual Studio, .NET Runtime 등 시스템 구성요소를 직접 삭제하지 않는다.
- 프로젝트 내부의 재생성 가능한 산출물만 격리 대상으로 취급한다.
- 위험도와 분석 신뢰도를 별도로 표시한다.
- 백그라운드 데몬은 필수 기능이 아니라 확장 기능이다.

---

## 2. CLI 전체 명령어 구조

```text
libra
├─ init
├─ scan
├─ summary
├─ projects
├─ resources
├─ explain
├─ impact
├─ plan
├─ clean
├─ restore
├─ transactions
├─ export
└─ daemon
   ├─ start
   ├─ status
   └─ stop
```

### 2.1 공통 옵션

| 옵션 | 설명 |
|---|---|
| `--config <path>` | 사용할 설정 파일 경로 |
| `--json` | 결과를 JSON으로 출력 |
| `--verbose` | 상세 로그 출력 |
| `--no-color` | ANSI 색상 제거 |
| `--dry-run` | 실제 변경 없이 실행 결과만 확인 |
| `--yes` | 대화형 확인 생략 |
| `--root <path>` | 분석할 프로젝트 루트 지정 |
| `--type <type>` | 프로젝트 또는 리소스 종류 필터 |
| `--risk <level>` | 위험도 필터 |
| `--project <path>` | 특정 프로젝트만 분석 |
| `--drive <drive>` | 특정 드라이브만 조회 |

---

## 3. 명령어별 기능 명세

## 3.1 `libra init`

설정 파일과 로컬 데이터베이스를 초기화한다.

```bash
libra init
libra init --config .libra.yaml
```

### 주요 기능

- 프로젝트 스캔 루트 등록
- 제외 경로 등록
- SQLite 데이터베이스 생성
- 격리 디렉터리 설정
- 시스템 보호 경로 기본 등록
- 기본 위험도 및 stale 기준 설정

### 설정 파일 예시

```yaml
version: 1

project_roots:
  - C:\Users\user\source
  - D:\Projects

exclude:
  - node_modules
  - .next
  - dist
  - build
  - bin
  - obj
  - .git
  - .libra-quarantine
  - C:\Windows
  - C:\Program Files
  - C:\Program Files (x86)
  - System Volume Information
  - .git\objects

scan:
  max_depth: 20
  follow_reparse_points: false
  stale_days: 90

cleanup:
  default_mode: dry-run
  quarantine_days: 7
```

### 우선순위

- **P0 필수**

> 2026-07-21 issue #36: config 기본 exclude와 중첩 path-segment matcher를
> 구현했다. project detector가 root에서 산출물을 직접 등록하므로 walk가
> `node_modules`/`dist` 내부로 내려가지 않아도 소유 Resource와 크기 측정은
> 유지된다.

---

## 3.2 `libra scan`

프로젝트, 개발 리소스, 빌드 산출물을 탐지하고 의존성 정보를 갱신한다.

```bash
libra scan
libra scan --root D:\Projects
libra scan --full
```

### 주요 기능

1. 설정된 프로젝트 루트 순회
2. 프로젝트 파일 탐지
3. 빌드 산출물과 대용량 디렉터리 탐지
4. Windows SDK 및 개발 도구 탐지
5. 프로젝트와 리소스 간 의존성 분석
6. 결과를 SQLite에 저장
7. 권한 오류와 미분석 경로 기록

### 탐지할 프로젝트

| 파일 또는 디렉터리 | 분류 |
|---|---|
| `.sln` | Visual Studio Solution |
| `.vcxproj` | MSBuild C++ 프로젝트 |
| `.csproj` | MSBuild .NET 프로젝트 |
| `package.json` | Node.js 프로젝트 |
| `.git` | Git 저장소 |

### 탐지할 프로젝트 산출물

- `node_modules`
- `bin`
- `obj`
- `build`
- `dist`
- `.next`
- `out`
- `Debug`
- `Release`

### 출력 예시

```text
Scan completed

Roots scanned:       2
Projects found:      18
Resources found:     42
Files inspected:     138,204
Logical size:        124.6GB
Potential cleanup:   18.2GB
Warnings:            3
```

### 우선순위

- **P0 필수**

---

## 3.3 `libra summary`

개발 관련 저장공간 현황과 정리 가능 용량을 요약한다.

```bash
libra summary
libra summary --drive C:
libra summary --type sdk
libra summary --json
```

### 주요 기능

- 프로젝트 수와 총 용량
- 리소스 종류별 용량
- 드라이브별 사용량
- 위험도별 정리 후보 용량
- 최근 스캔 시각
- 미분석 경로와 경고 수

### 출력 예시

```text
C: drive developer storage

Windows SDKs             11.6GB
Visual Studio tools      24.2GB
.NET SDKs                 5.4GB
Node project artifacts   18.1GB
MSBuild outputs           7.6GB
Unknown large folders    14.0GB

Safely reclaimable        9.7GB
Needs review             12.4GB
Blocked                  58.8GB
```

### 우선순위

- **P0 필수**

---

## 3.4 `libra projects`

발견된 프로젝트 목록과 활동 상태를 보여준다.

```bash
libra projects
libra projects --type node
libra projects --drive D:
libra projects --status stale
```

### 출력 정보

- 프로젝트명
- 프로젝트 경로
- 프로젝트 종류
- 드라이브
- 논리 크기
- 마지막 수정 시각
- 마지막 관찰 시각
- 활동 상태
- 연결된 리소스 수

### 프로젝트 상태

| 상태 | 의미 |
|---|---|
| `ACTIVE` | 최근 수정되었거나 최근 사용 근거가 있음 |
| `STALE` | 설정된 기간 이상 수정되지 않음 |
| `ARCHIVED` | 사용자가 보관 프로젝트로 지정 |
| `UNKNOWN` | 활동 여부 판단 불가 |

### 우선순위

- **P0 필수**

---

## 3.5 `libra resources`

발견된 SDK, 개발 도구, 캐시, 빌드 산출물 목록을 보여준다.

```bash
libra resources
libra resources --type windows-sdk
libra resources --type build-output
libra resources --risk review
```

### 지원 리소스 종류

| 리소스 | MVP 지원 |
|---|---|
| Windows SDK | P0 |
| Visual Studio 설치 정보 | P0 |
| MSBuild | P0 |
| .NET SDK | P0 |
| 프로젝트 `node_modules` | P0 |
| 프로젝트 `bin`, `obj`, `build`, `dist` | P0 |
| npm·pnpm 전역 캐시 | P1 |
| Docker 이미지·빌드 캐시 | P2 |

### 출력 정보

- 리소스명
- 종류
- 버전
- 경로
- 논리 크기
- 연결 프로젝트 수
- 재생성 가능 여부
- 위험도
- 분석 신뢰도

### 우선순위

- **P0 필수**

---

## 3.6 `libra explain`

특정 프로젝트 또는 리소스가 무엇이며 왜 존재하는지 설명한다.

```bash
libra explain windows-sdk:10.0.22621.0
libra explain "D:\Projects\OldWeb\node_modules"
libra explain project:"D:\Projects\GameClient"
```

### 필수 출력 정보

- 항목 종류
- 실제 경로
- 버전
- 크기
- 연결 프로젝트 또는 리소스
- 의존성 근거
- 재생성 가능 여부
- 삭제 예상 영향
- 복구 방법
- 위험도
- 분석 신뢰도
- 분석하지 못한 범위

### 출력 예시

```text
Resource: Windows SDK 10.0.22621.0
Path: C:\Program Files (x86)\Windows Kits\10
Size: 3.1GB

Used by:
- D:\Projects\GameClient
  Evidence: DECLARED
  Source: GameClient.vcxproj
  Property: WindowsTargetPlatformVersion

Expected impact:
- Existing executable launch: LOW
- Rebuild: HIGH
- Visual Studio debugging: HIGH when rebuild occurs

Risk: BLOCKED
Confidence: 88%

Unverified:
- Unchecked Git branches
- Disconnected drives
- Remote CI configurations
```

### 우선순위

- **P0 필수**

---

## 3.7 `libra impact`

특정 리소스를 제거했을 때 영향을 받는 프로젝트와 작업을 분석한다.

```bash
libra impact windows-sdk:10.0.22621.0
libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"
```

### 영향 분류

| 분류 | 의미 |
|---|---|
| `RUN` | 이미 빌드된 프로그램 직접 실행 |
| `BUILD` | 프로젝트 재빌드 |
| `DEBUG` | IDE 디버깅 |
| `RESTORE` | 의존성 재설치 또는 SDK 복구 |
| `CI` | 발견된 CI 설정 |
| `UNKNOWN` | 영향 판단 불가 |

### 출력 예시

```text
Affected projects: 2

D:\Projects\GameClient
RUN      likely unaffected
BUILD    expected to fail
DEBUG    expected to fail if build runs
RESTORE  reinstall SDK through Visual Studio Installer

D:\Archive\OldLauncher
RUN      unknown
BUILD    expected to fail
DEBUG    unknown
RESTORE  reinstall SDK
```

### 우선순위

- **P0 핵심 기능**
- Day 4까지 반드시 완성

---

## 3.8 `libra plan`

목표 용량과 위험도에 맞게 정리 계획을 생성한다.

```bash
libra plan
libra plan --target 10GB
libra plan --risk safe
libra plan --project D:\Projects\OldWeb
```

### 주요 기능

- 정리 후보를 `SAFE`, `REVIEW`, `BLOCKED`로 분류
- 목표 용량을 만족하도록 후보 선택
- 예상 확보 용량 계산
- 재생성 방법과 위험 근거 표시
- 시스템 리소스 자동 제외
- 계획 ID 생성

### 출력 예시

```text
Plan ID: plan-20260717-001
Target: 10.0GB
Selected: 11.4GB

SAFE
[1] 4.2GB D:\Projects\OldWeb\node_modules
[2] 2.7GB D:\Projects\GameClient\build
[3] 1.5GB D:\Projects\App\bin
[4] 1.2GB D:\Projects\App\obj

REVIEW
[5] 1.8GB pnpm unreferenced store

BLOCKED
[ ] 3.1GB Windows SDK 10.0.22621.0
    Used by GameClient
```

### 우선순위

- **P0 필수**

---

## 3.9 `libra clean`

생성된 정리 계획을 실행한다.

```bash
libra clean --plan plan-20260717-001
libra clean --plan plan-20260717-001 --execute
```

> 현재 계약: 플래그 없이 실행하면 dry-run이다. `--execute`가 실제 격리를
> 활성화하고 `--yes`는 대화형 확인만 생략한다. 실제 삭제는 하지 않고 같은
> volume의 `.libra-quarantine/<transaction-id>`로 이동한다.

### 주요 기능

- 기본 dry-run과 명시적 `--execute`
- explicit `OWNS` 관계를 가진 `SAFE` 산출물만 처리
- 실행 전 path/type/size/mtime/owner/Git/reparse/protected path 재검증
- 이동 전 disk manifest 기록, 이동 후 manifest와 DB transaction 갱신
- item별 `MOVED`/`SKIPPED`/`FAILED`, 부분 성공 상태 제공
- restore 시 기존 원본을 절대 덮어쓰지 않음

### 직접 처리 금지 대상

- Windows SDK
- Visual Studio 구성요소
- .NET Runtime
- Windows App Runtime
- Docker Volume
- 데이터베이스
- 사용자 문서
- Git object
- `.env`
- 인증서 및 키
- 알 수 없는 대용량 폴더

### 우선순위

- **P0 후반**
- read-only 핵심 기능 완성 후 구현

---

## 3.10 `libra restore`

격리한 파일을 원래 위치로 복구한다.

```bash
libra restore tx-20260717-001
```

### 주요 기능

- 원래 위치 충돌 확인
- 자동 덮어쓰기 금지
- 격리 manifest 검증
- 원래 위치로 이동
- DB transaction 상태 변경
- 일부 복구 실패 시 상세 출력

### 우선순위

- **P0 후반**

---

## 3.11 `libra transactions`

정리와 복구 이력을 확인한다.

```bash
libra transactions
libra transactions --status quarantined
```

### 출력 정보

- transaction ID
- 실행 시각
- 계획 ID
- 처리 항목 수
- 격리 용량
- 현재 상태
- 복구 가능 여부

### 우선순위

- **P0 후반**

---

## 3.12 `libra export`

분석 결과를 외부 파일로 내보낸다.

```bash
libra export --format json
libra export --format markdown
libra export --format json --output report.json
```

### 지원 형식

- JSON
- Markdown

### 활용

- 발표 자료
- 버그 리포트
- 후속 GUI
- AI 분석 입력
- 팀원 간 개발환경 비교

### 우선순위

- **P1**

---

## 3.13 `libra daemon`

설정된 프로젝트 루트의 변경 사항을 감시하고 인덱스를 증분 갱신한다.

```bash
libra daemon start
libra daemon status
libra daemon stop
```

### 감시할 이벤트

- 파일 생성
- 파일 삭제
- 파일명 변경
- 파일 크기 변경
- 프로젝트 추가 및 제거
- 리소스 디렉터리 변경

### 하지 않을 기능

- 모든 파일 읽기 추적
- 모든 프로세스 I/O 추적
- ETW 기반 전체 provenance 수집
- 실시간 자동 삭제
- 파일별 직접 의존성 edge 생성

### 우선순위

- **P2 확장 기능**
- 핵심 기능과 정리 기능이 안정된 후 구현

---

## 4. 위험도 및 근거 모델

## 4.1 위험도

| 위험도 | 설명 | 예시 |
|---|---|---|
| `SAFE` | 프로젝트 내부의 재생성 가능한 산출물 | `node_modules`, `bin`, `obj`, `dist` |
| `REVIEW` | 현재 참조는 없지만 다시 필요할 수 있음 | 오래된 SDK, 전역 패키지 캐시 |
| `BLOCKED` | 현재 참조 중이거나 사용자 데이터 위험이 있음 | 사용 중인 SDK, Runtime, Docker Volume |

## 4.2 근거 종류

| 근거 | 설명 |
|---|---|
| `DECLARED` | 프로젝트 설정에 직접 선언 |
| `RESOLVED` | 빌드 도구가 최종 해석 |
| `OBSERVED` | 실제 사용 또는 데몬에서 관찰 |
| `INFERRED` | 경로, 파일명, 수정 시점으로 추론 |
| `UNKNOWN` | 충분한 정보 없음 |

## 4.3 신뢰도

신뢰도는 실제 확률이 아니라 분석 범위 충족도를 나타낸다.

예시 가중치:

```text
RESOLVED evidence       +35
DECLARED evidence       +25
OBSERVED evidence       +20
Known adapter           +10
Recent successful scan  +10

Unscanned drive         -15
Unresolved variable     -15
Permission denied       -20
Missing build tool      -10
Unknown project type    -20
```

---

## 5. 팀원 역할

| 구분 | 환경 | 주 역할 | 코드 소유 영역 |
|---|---|---|---|
| **Windows A** | Windows | 플랫폼·스캔·DB | `scanner`, `store`, `config`, `safety` |
| **Windows B** | Windows | Windows 의존성 분석 | `windowsdk`, `msbuild`, `dotnet`, graph |
| **Mac C** | macOS | CLI·출력·Node·QA | `cmd`, `output`, `node`, `testdata`, 문서 |

### Windows A — 플랫폼 및 데이터 담당

- 파일 및 디렉터리 스캔
- 크기 계산
- 경로 정규화
- 권한 오류 처리
- junction 및 symbolic link 안전 처리
- SQLite schema와 repository
- transaction 및 quarantine 기반
- 여유가 생길 경우 데몬

### Windows B — Windows 의존성 담당

- `.sln`, `.vcxproj`, `.csproj` 탐지
- Windows SDK 설치 버전 탐지
- `vswhere.exe` 연동
- MSBuild 위치 탐지
- `.vcxproj` 속성 파싱
- `Directory.Build.props` 탐색
- .NET SDK 탐지
- 프로젝트와 SDK 간 dependency edge 생성
- impact 규칙

### Mac C — CLI 및 제품 흐름 담당

- Cobra 명령 구조
- 공통 플래그
- 텍스트 및 JSON 출력
- Node 프로젝트 탐지
- `package.json` 파싱
- `node_modules`, `dist`, `.next`, `build` 분류
- fixture 및 Golden output 테스트
- README와 발표 데모
- Windows 전용 기능의 mock 출력과 공통 parser 테스트

---

## 6. 일자별 사람별 개발 일정

## Day 1 — 프로젝트 골격 및 계약 확정

| Windows A | Windows B | Mac C |
|---|---|---|
| Go 프로젝트 및 SQLite 연결 | Domain 모델 정의 | Cobra 명령 scaffold |
| DB schema 및 migration 작성 | Adapter 인터페이스 정의 | 공통 옵션 정의 |
| config loader 구조 작성 | Project, Resource, Dependency, Evidence 모델 | 출력 formatter 기본 구현 |
| scanner 인터페이스 정의 | MSBuild fixture 초안 | README 실행 방법 초안 |
| CI의 Windows 작업 확인 | 공통 parser 테스트 구조 생성 | CI의 macOS 작업 확인 |

### 공동 완료 조건

- `go run . --help` 성공
- `go test ./...` Windows/macOS 성공
- `go build ./...` Windows/macOS 성공
- SQLite 파일 생성 가능
- `scan`, `summary`, `explain`, `impact` 명령이 도움말에 표시

---

## Day 2 — 파일 스캔 및 프로젝트 탐지

| Windows A | Windows B | Mac C |
|---|---|---|
| 재귀 디렉터리 스캐너 | `.sln`, `.vcxproj`, `.csproj` 탐지 | `package.json` 탐지 |
| 파일·디렉터리 논리 크기 계산 | 프로젝트 루트 판별 | Node 산출물 탐지 |
| 접근 권한 오류 수집 | Git 저장소 판별 | `projects` 출력 |
| reparse point 무시 | manifest 메타데이터 생성 | `summary` mock→실데이터 연결 |
| 스캔 결과 SQLite 저장 | MSBuild fixture 단위 테스트 | Node fixture와 Golden test |

### 공동 완료 조건

```bash
libra scan --root ./testdata
libra projects
libra summary
```

- fixture의 프로젝트 종류가 정상 출력
- `node_modules`, `bin`, `obj`, `build`, `dist` 크기 계산
- 권한 오류가 전체 스캔을 중단시키지 않음

---

## Day 3 — Windows SDK 및 개발 리소스

| Windows A | Windows B | Mac C |
|---|---|---|
| 알려진 리소스 경로 크기 집계 | Windows SDK 탐지 | `resources` 명령 |
| Resource DB 저장 | `vswhere.exe` 연동 | `explain` 출력 모델 |
| Windows 경로 정규화 | Visual Studio·MSBuild 탐지 | `--json` 출력 |
| 시스템 경로 안전 분류 | `dotnet --list-sdks` 파싱 | Adapter 실패·미지원 출력 |
| 실제 Windows 장비 스캔 지원 | `.vcxproj` SDK 속성 파싱 | fixture 확장 및 문서화 |

### 공동 완료 조건

```bash
libra resources --type windows-sdk
libra explain windows-sdk:<version>
```

- 설치된 Windows SDK 버전별 목록
- SDK 크기와 경로 출력
- 실제 또는 fixture 프로젝트와 연결 근거 출력

---

## Day 4 — 의존성 그래프 및 영향 분석

| Windows A | Windows B | Mac C |
|---|---|---|
| dependency·evidence DB 구현 | 프로젝트→Windows SDK edge | `impact` 명령 구현 |
| graph 조회 쿼리 | 프로젝트→.NET SDK edge | RUN·BUILD·DEBUG 출력 |
| C와 D 드라이브 간 관계 저장 | 프로젝트→산출물 edge | 위험도·신뢰도 formatter |
| 프로젝트·리소스 역방향 조회 | impact 판단 규칙 | Golden output 테스트 |
| 조회 성능 점검 | 실제 Windows 환경 검증 | 사용자 문구 및 경고 정리 |

### 공동 완료 조건

```bash
libra impact windows-sdk:<version>
```

반드시 다음 내용을 보여야 한다.

- C 드라이브 리소스를 참조하는 D 드라이브 프로젝트
- 설정 파일과 속성명
- 기존 실행 파일 영향
- 재빌드 영향
- 디버깅 영향
- 복구 방법
- 위험도와 신뢰도

> Day 4까지 이 기능이 완성되지 않으면 `clean`과 `daemon` 구현을 시작하지 않는다.

### 현재 상태 (2026-07-20)

Day 4의 핵심 경로인 MSBuild 프로젝트 탐지 → Windows SDK/.NET SDK 의존성 분석
→ dependency/Evidence 저장 → `explain`/`impact` 출력은 구현 및 E2E 검증을
완료했다. 중앙 `RiskPolicy`도 완전한 `CleanupEvidence`에서만 SAFE를 반환한다.

> 갱신(2026-07-20): reparse point와 Git tracked 원본 부재 증거 수집을
> Node/MSBuild 양쪽에 구현했다(`internal/app/project_detector_adapters.go`의
> `projectArtifactCleanupEvidence`). git tracked 파일 탐지는 실제
> `git init`/`git add` fixture로 E2E 검증 완료. reparse point 탐지는 코드는
> 붙였지만, 이 개발 환경에 symlink 생성 권한(Developer 모드/관리자)이 없어서
> 로컬에서는 테스트가 스킵됐다 — **Day 5 clean 실행 전에 실제 Windows
> 환경(권한 있는 곳)에서 한 번은 이 경로를 직접 검증해야 한다.**

---

## Day 5 — 정리 계획 및 안전한 격리

### 진행 상태 (2026-07-20)

- plan greedy 선택과 immutable snapshot 저장 완료
- scan에서 `PROJECT --OWNS--> RESOURCE`와 OBSERVED Evidence 저장 완료
- 실행 직전 전체 filesystem/safety 재검증 완료
- manifest-first quarantine와 transaction/item 부분 실패 상태 완료
- `transactions` 조회 및 원본 충돌 없는 `restore` 완료
- 임시 디렉터리 실제 이동·복구 통합 테스트 완료
- 남은 항목은 Windows 실제 junction/ACL/hidden 통합 검증, typed exit code,
  공통 JSON envelope와 명시적 purge다.

| Windows A | Windows B | Mac C |
|---|---|---|
| cleanup plan DB | 재생성 가능 규칙 | `plan --target` 구현 |
| transaction repository | SAFE·REVIEW·BLOCKED 분류 | dry-run 출력 |
| 대상 크기 재검증 | 시스템 리소스 차단 근거 | 정리 후보 선택 UX |
| quarantine 이동 기반 | 복구 명령 생성 | `clean` 명령 연결 |
| restore 기반 | cleanup 안전성 코드 리뷰 | 통합 시나리오 테스트 |

### 공동 완료 조건

```bash
libra plan --target 1GB
libra clean --plan <id>
libra clean --plan <id> --execute --yes
libra transactions
libra restore --transaction <transaction-id>
```

- 시스템 SDK는 구조적으로 plan에서 제외
- explicit owner project 내부 산출물만 격리
- 격리 후 원래 위치 복구 및 충돌 시 overwrite 금지

---

## Day 6 — 통합 테스트 및 안전성 보강

| Windows A | Windows B | Mac C |
|---|---|---|
| 대규모 fixture 성능 테스트 | 오탐·누락 사례 수정 | 신규 사용자 관점 테스트 |
| DB batch 처리 | 미해석 변수 경고 | CLI 도움말 정리 |
| scanner 취소 및 오류 처리 | 분석 범위 표시 | README 완성 |
| 경로 재검증 | 실제 Windows 장비 비교 | 발표 fixture 생성 |
| cleanup 코드 리뷰 | cleanup 코드 리뷰 | backup 데모 시나리오 |

### 공동 완료 조건

- 새 Windows 장비에서 README만 보고 실행 가능
- `scan → summary → explain → impact → plan` 시나리오 성공
- `clean → restore` 성공
- 치명적 crash 없음
- 시스템 경로가 정리되지 않음

---

## Day 7 — 발표 버전 고정 및 확장 기능

### 오전 공통 작업

- 새로운 P0 기능 추가 금지
- 핵심 버그만 수정
- release binary 생성
- 최종 fixture와 DB snapshot 고정
- 백업 데모 영상 녹화
- 알려진 한계 문서 작성

### 핵심 기능이 안정된 경우

| Windows A | Windows B | Mac C |
|---|---|---|
| 파일 watcher와 dirty queue | 변경 리소스 재분석 | `daemon start/status/stop` CLI |
| 증분 크기 계산 | 변경 프로젝트 재연결 | 이벤트 목록 출력 |
| watcher 오류 복구 | Windows 통합 테스트 | 데몬 사용 문서 |
| 성능·메모리 측정 | 오탐 여부 확인 | 발표 흐름 반영 |

### 데몬보다 우선할 P1 기능

1. MSBuild preprocess 분석
2. Markdown report
3. `Directory.Build.props` 조건 개선
4. pnpm 전역 store 분석
5. Docker `system df` Adapter

---

## 7. 일자별 핵심 마일스톤

| 날짜 | 핵심 산출물 |
|---|---|
| Day 1 | 모든 OS에서 빌드되는 Cobra 프로젝트 |
| Day 2 | 프로젝트와 산출물을 찾는 scanner |
| Day 3 | Windows SDK와 개발 리소스 탐지 |
| Day 4 | 프로젝트와 SDK 간 impact 분석 |
| Day 5 | 목표 용량 정리 계획, 격리, 복구 |
| Day 6 | 실제 장비 통합 테스트와 안전성 보강 |
| Day 7 | 발표 버전 고정, 여유 시 데몬 |

---

## 8. 기능 우선순위

## P0 — 반드시 구현

- `init`
- `scan`
- `summary`
- `projects`
- `resources`
- `explain`
- `impact`
- `plan`
- `clean --dry-run`
- 프로젝트 산출물 격리
- `restore`
- Windows SDK 탐지
- Visual Studio/MSBuild 탐지
- .NET SDK 탐지
- Node 프로젝트 탐지
- SQLite 저장
- 위험도·신뢰도·근거 표시

## P1 — 핵심 기능 완료 후

- MSBuild preprocess
- `Directory.Build.props` 고급 분석
- Markdown·JSON export
- pnpm 전역 store
- 실제 빌드 검증 명령
- 스캔 결과 증분 최적화

## P2 — 여유가 있을 때

- 백그라운드 데몬
- NTFS USN Journal
- Docker Adapter
- ETW 기반 개발 프로세스 관찰
- 인터랙티브 TUI
- 로컬 웹 대시보드

---

## 9. 일정 지연 시 기능 컷 순서

다음 순서로 제거한다.

1. 백그라운드 데몬
2. Docker Adapter
3. pnpm 전역 저장소
4. Markdown report
5. MSBuild binary log
6. 실제 build 검증
7. 복잡한 MSBuild 조건식
8. cleanup UI 개선

끝까지 유지한다.

- 프로젝트 스캔
- Windows SDK 탐지
- 프로젝트와 SDK 연결
- `summary`
- `explain`
- `impact`
- `plan`
- `dry-run`
- 최소한의 격리와 복구

---

## 10. 권장 저장소 구조

```text
libra/
├─ main.go
├─ go.mod
├─ go.sum
├─ cmd/
│  ├─ root.go
│  ├─ init.go
│  ├─ scan.go
│  ├─ summary.go
│  ├─ projects.go
│  ├─ resources.go
│  ├─ explain.go
│  ├─ impact.go
│  ├─ plan.go
│  ├─ clean.go
│  ├─ restore.go
│  ├─ transactions.go
│  ├─ export.go
│  └─ daemon.go
├─ internal/
│  ├─ app/
│  ├─ domain/
│  ├─ scanner/
│  ├─ adapter/
│  │  ├─ windowsdk/
│  │  ├─ visualstudio/
│  │  ├─ msbuild/
│  │  ├─ dotnet/
│  │  └─ node/
│  ├─ store/
│  │  └─ sqlite/
│  ├─ config/
│  ├─ output/
│  ├─ cleanup/
│  └─ safety/
├─ testdata/
│  ├─ filesystem/
│  ├─ msbuild/
│  ├─ dotnet/
│  └─ node/
├─ docs/
├─ scripts/
└─ .github/
   └─ workflows/
```

### 구조 원칙

- `cmd`에는 비즈니스 로직을 넣지 않는다.
- Windows 전용 탐지와 공통 parser를 분리한다.
- macOS에서도 `go test ./...`가 성공해야 한다.
- 정리 로직은 `cleanup`과 `safety`에 집중시킨다.
- Adapter는 파일을 직접 삭제하지 않는다.

---

## 11. Git 및 협업 규칙

### 브랜치 예시

```text
main
feature/scanner
feature/windows-sdk
feature/summary-output
feature/impact
fix/path-normalization
```

### 기본 원칙

- `main`은 항상 빌드 가능하게 유지한다.
- 브랜치는 최대 하루 안에 merge한다.
- 매일 점심과 저녁에 통합한다.
- 큰 PR 하나보다 작은 PR 여러 개를 만든다.
- DB schema 변경은 세 명이 합의한다.
- cleanup 관련 PR은 최소 2명이 리뷰한다.
- 새 기능에는 fixture 또는 단위 테스트를 포함한다.

### PR 체크리스트

```text
[ ] gofmt 적용
[ ] go test ./... 성공
[ ] go vet ./... 성공
[ ] Windows 또는 macOS 실행 결과 첨부
[ ] 새 기능 테스트 포함
[ ] CLI 변경 시 실행 예시 포함
[ ] cleanup 코드라면 2인 이상 리뷰
```

---

## 12. 최종 데모 흐름

```bash
libra scan
libra summary
libra resources --type windows-sdk
libra explain windows-sdk:10.0.22621.0
libra impact windows-sdk:10.0.22621.0
libra plan --target 5GB
libra clean --plan <plan-id>
libra clean --plan <plan-id> --execute
libra restore <transaction-id>
```

데몬이 완성된 경우:

```bash
libra daemon start
libra daemon status
libra summary
```

### 데모에서 보여줄 핵심

1. C 드라이브의 SDK와 D 드라이브 프로젝트 사이의 연결
2. 실행·빌드·디버깅 영향의 구분
3. `SAFE`, `REVIEW`, `BLOCKED` 분류
4. 시스템 구성요소가 자동 삭제되지 않는 안전성
5. 프로젝트 산출물 격리와 복구
6. 여력이 있다면 실시간 저장공간 변화 반영

---

## 13. 완료 기준

프로젝트는 다음 세 질문에 답할 수 있으면 핵심 목표를 달성한 것으로 본다.

### 질문 1

> Windows SDK 10.0.19041.0을 제거해도 되는가?

필수 답변:

- 크기와 경로
- 연결 프로젝트
- 의존성 근거
- 기존 실행 영향
- 재빌드 영향
- 위험도와 신뢰도
- 공식 복구 또는 제거 방법

### 질문 2

> C 드라이브의 이 리소스를 제거하면 D 드라이브 프로젝트가 영향을 받는가?

필수 답변:

- 영향을 받는 프로젝트
- 프로젝트 경로와 드라이브
- 설정 파일 및 속성
- 영향 종류
- 분석하지 못한 범위

### 질문 3

> 안전하게 5GB를 확보하려면 무엇을 정리해야 하는가?

필수 답변:

- SAFE 후보
- REVIEW 후보
- BLOCKED 후보
- 예상 확보 용량
- dry-run 결과
- 최소 하나의 격리 및 복구 성공
