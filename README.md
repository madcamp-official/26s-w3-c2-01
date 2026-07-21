# 26s-w3-c2-01
몰입캠프 26s-w3-c2-03 프로젝트 repository

## 실행 방법

### 준비물

- Go 1.25 이상 (`go.mod`의 `go 1.25.0` 기준)

### 빌드 및 실행

```bash
git clone <repo-url>
cd 26s-w3-c2-01

# 소스에서 바로 실행
go run . --help

# 또는 바이너리로 빌드해서 실행
go build -o libra .
./libra --help
```

### 사용 가능한 명령 (Day 4 기준)

구현 진행 상황은 `docs/libra_cli_commands_and_schedule.md`의 일정을 따릅니다.

| 명령 | 상태 |
|---|---|
| `libra init` | 구현됨 — 설정 파일과 SQLite DB를 생성합니다 |
| `libra scan` | 구현됨 — 프로젝트·리소스를 탐지해 SQLite에 저장합니다 (Windows SDK 등 시스템 리소스 탐지는 Windows 환경에서만 동작하고, 다른 플랫폼에서는 명시적인 미지원 경고로 표시됩니다) |
| `libra summary` | 구현됨 — 실제 스캔 결과로 저장공간 현황을 요약합니다 (`--json` 지원) |
| `libra projects` | 구현됨 — 발견된 프로젝트 목록과 활동 상태를 보여줍니다 |
| `libra resources` | 구현됨 — 발견된 SDK·도구·빌드 산출물과 다차원 confidence·구조화된 risk reason을 보여줍니다 |
| `libra issues` | 구현됨 — 최신 또는 지정한 스캔의 경고·오류를 조회합니다 (`--scan`, `--code`, `--severity`, `--json` 지원) |
| `libra explain <target>` | 구현됨 — 프로젝트 또는 리소스 하나를 설명합니다 |
| `libra impact <target>` | 구현됨 — 리소스를 제거했을 때 영향받는 프로젝트를 보여줍니다 |
| `libra plan` | 구현됨 — SAFE 산출물을 결정적 순서로 선택하고 snapshot을 저장합니다 |
| `libra clean --plan <id>` | 구현됨 — 기본 dry-run, `--execute`로 같은 volume quarantine에 격리합니다 |
| `libra restore --transaction <id>` | 구현됨 — 원본 충돌 없이 격리 항목을 복구합니다 |
| `libra transactions` | 구현됨 — 격리·복구 transaction과 item 상태를 보여줍니다 |
| `libra export` | 구현됨 — 최신 scan의 project/resource/issue/transaction을 JSON 또는 Markdown으로 내보냅니다. |
| `libra purge --transaction <id>` | 구현됨 — 보존기간을 지난 quarantine을 기본 dry-run으로 검증하고 `--execute`에서만 영구 삭제합니다. |
| `libra daemon start/status/stop` | 구현됨 — 설정된 project root의 변경을 polling하고 기존 scan을 실행합니다. 자동 정리는 하지 않습니다. |

```bash
go run . scan --root ./testdata
go run . summary
go run . --json summary
go run . resources --type node-modules
go run . explain windows-sdk:10.0.22621.0
go run . explain "D:\Projects\OldWeb\node_modules"
go run . explain project:"D:\Projects\GameClient"
go run . impact windows-sdk:10.0.22621.0
```

**알려진 한계:** MSBuild 프로젝트의 Windows SDK/.NET SDK 의존성 분석은 실제 `scan` 파이프라인에 연결되어 `explain`/`impact`에서 조회할 수 있습니다. 다만 RUN·DEBUG·CI 영향 규칙은 아직 제한적이며, 프로젝트 산출물은 project root와 알려진 output path만 확인된 상태입니다. reparse point와 Git tracked 원본 부재까지 검증되기 전에는 안전하게 `REVIEW`로 유지됩니다.

### 개발 중 검증

PR을 올리기 전에 아래를 통과시켜야 합니다 (자세한 규칙은 `docs/libra_collaboration_rules.md` 참고).

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./...
```

---

# Libra 프로젝트 기획서

## 1. 프로젝트 개요

### 1.1 한 줄 소개

> **로컬 컴퓨터의 개발 프로젝트와 SDK·도구·캐시·빌드 산출물 사이의 의존 관계를 분석해, 무엇이 공간을 차지하고 있으며 삭제 시 어떤 영향이 생기는지 설명하는 CLI 도구**

### 1.2 해결하려는 문제

개발자의 컴퓨터에는 시간이 지날수록 다음 데이터가 누적된다.

* 여러 버전의 Windows SDK와 .NET SDK
* Visual Studio 및 MSVC Toolset
* 프로젝트별 `node_modules`
* `bin`, `obj`, `build`, `dist` 등의 빌드 산출물
* npm·pnpm 등의 패키지 캐시
* Gradle·Android SDK 캐시
* Docker 이미지와 빌드 캐시
* 더 이상 사용하지 않는 과거 프로젝트

기존 저장공간 분석기는 어떤 폴더가 큰지는 보여주지만 다음 질문에는 제대로 답하기 어렵다.

* 이 Windows SDK를 어떤 프로젝트가 사용하는가?
* C 드라이브의 SDK를 지우면 D 드라이브 프로젝트가 영향을 받는가?
* 이 폴더는 원본 파일인가, 다시 생성할 수 있는 산출물인가?
* 10GB를 확보하려면 무엇부터 정리하는 것이 안전한가?
* 삭제 후 문제가 생기면 복구할 수 있는가?

Libra는 파일 크기만 나열하지 않고, **프로젝트와 개발 리소스 사이의 관계와 판단 근거**를 함께 제공한다.

---

# 2. 프로젝트 목표

## 2.1 핵심 목표

1. 여러 드라이브에서 개발 프로젝트를 발견한다.
2. 시스템에 설치된 개발 리소스와 대용량 산출물을 발견한다.
3. 프로젝트가 요구하는 SDK·도구 버전을 정적 분석한다.
4. 프로젝트와 리소스 사이의 의존성 그래프를 만든다.
5. 특정 파일이나 SDK의 삭제 영향을 설명한다.
6. 확보 가능한 공간을 위험도별로 추천한다.
7. 안전한 대상에 한해 격리 및 복구 기능을 제공한다.
8. 여력이 있다면 변경 사항을 증분 반영하는 백그라운드 데몬을 구현한다.

## 2.2 이번 주에 해결하지 않을 문제

* 운영체제 전체 파일의 완전한 의미적 의존성 분석
* 커널 드라이버나 파일시스템 minifilter 구현
* 모든 파일 읽기·쓰기 이벤트 추적
* macOS와 Linux 지원
* 모든 언어와 패키지 매니저 지원
* Windows SDK나 Visual Studio 구성요소 직접 강제 삭제
* 사용자 문서, 데이터베이스, Docker Volume 등의 자동 삭제
* AI가 판단해서 자동으로 파일을 삭제하는 기능
* GUI 애플리케이션
* 중복 파일의 block-level deduplication

---

# 3. 핵심 제품 원칙

## 3.1 Read-only first

기본 동작은 항상 분석이다.

```bash
libra scan
libra explain windows-sdk:10.0.22621.0
libra impact "C:\Program Files (x86)\Windows Kits\10"
libra plan --target 10GB
```

사용자가 명시적으로 실행하지 않는 한 어떠한 파일도 변경하지 않는다.

## 3.2 시스템 구성요소 직접 삭제 금지

다음 경로와 시스템 구성요소는 MVP에서 직접 삭제하지 않는다.

* `C:\Windows`
* `C:\Program Files`
* `C:\Program Files (x86)`
* Windows SDK
* Visual Studio
* MSVC Toolset
* .NET Runtime
* Windows App Runtime
* 시스템 환경변수와 레지스트리

이들에 대해서는 다음만 제공한다.

* 크기
* 설치 버전
* 참조 프로젝트
* 삭제 예상 영향
* 공식 제거 방법
* 대체 버전
* 검증 필요 사항

Windows SDK가 프로젝트에서 요구되지만 설치되어 있지 않으면 MSBuild에서 `MSB8036` 오류가 발생할 수 있으므로, SDK는 단순 폴더 삭제가 아닌 설치 구성요소로 취급해야 한다.

## 3.3 판단과 근거를 함께 표시

모든 의존 관계에는 근거 종류가 붙는다.

| 근거         | 의미                   |
| ---------- | -------------------- |
| `DECLARED` | 프로젝트 설정 파일에 직접 선언됨   |
| `RESOLVED` | 빌드 도구가 최종적으로 해석한 값   |
| `OBSERVED` | 실제 실행이나 데몬에서 사용이 관찰됨 |
| `INFERRED` | 경로, 이름, 수정 시점 등으로 추론 |
| `UNKNOWN`  | 충분한 정보를 얻지 못함        |

예시:

```text
D:\Projects\GameClient
→ Windows SDK 10.0.22621.0

근거:
[DECLARED] GameClient.vcxproj
[RESOLVED] MSBuild preprocess 결과
[OBSERVED] 없음
```

## 3.4 안전과 신뢰도를 분리

위험도와 분석 신뢰도는 서로 다른 값이다.

```text
위험도: REVIEW
신뢰도: 91%
```

* 위험도는 삭제 시 피해 가능성을 뜻한다.
* 신뢰도는 분석 근거가 얼마나 충분한지를 뜻한다.

---

# 4. 타깃 사용자

## 4.1 주 사용자

* 여러 프로젝트를 동시에 개발하는 학생·개발자
* C 드라이브 용량이 자주 부족한 Windows 개발자
* Visual Studio, Android Studio, Node.js 등을 함께 사용하는 개발자
* 오래된 SDK와 개발 도구를 정리하기 어려운 사용자
* 프로젝트가 C와 D 드라이브에 흩어져 있는 사용자

## 4.2 대표 사용자 시나리오

### 시나리오 A: 오래된 Windows SDK 정리

```text
Windows SDK가 네 개 설치되어 있다.
낮은 버전을 삭제하고 싶지만 과거 프로젝트가 사용할까 걱정된다.
```

Libra의 답:

```text
Windows SDK 10.0.19041.0
크기: 2.4GB
현재 발견된 프로젝트 참조: 없음
과거 프로젝트 선언 참조: 1개
마지막 프로젝트 수정: 428일 전
기존 실행 파일 영향: 낮음
해당 프로젝트 재빌드 영향: 높음
판단: REVIEW
```

### 시나리오 B: 다른 드라이브에 미치는 영향

```text
C:\Program Files (x86)\Windows Kits\10\...
이 항목을 제거하면 D:\Projects\GameClient가 영향을 받는가?
```

Libra의 답:

```text
영향 받는 프로젝트: 1개

D:\Projects\GameClient
- GameClient.vcxproj가 10.0.22621.0을 지정
- 기존 game.exe 직접 실행: 영향 가능성 낮음
- 프로젝트 재빌드: 실패 예상
- Visual Studio F5 디버깅: 빌드가 발생하면 실패 예상
```

### 시나리오 C: 목표 용량 확보

```bash
libra plan --target 10GB
```

```text
목표: 10.0GB
추천 확보 공간: 12.6GB

SAFE
4.2GB  D:\Projects\OldWeb\node_modules
2.8GB  D:\Projects\GameClient\build
1.6GB  D:\Projects\App\bin + obj

REVIEW
4.0GB  사용 기록이 없는 Windows SDK 10.0.19041.0
```

---

# 5. 1주 MVP 범위

## 5.1 P0: 반드시 완성할 기능

### 플랫폼

* Windows 10·11
* NTFS 로컬 드라이브
* 관리자 권한 없이 실행
* 필요 기능이 관리자 권한을 요구하면 분석 결과에 표시

### 지원 프로젝트

* Visual Studio Solution: `.sln`
* C++ MSBuild: `.vcxproj`
* .NET MSBuild: `.csproj`
* Node.js: `package.json`
* Git 저장소: `.git`
* Python: `pyproject.toml`, `Pipfile`, `setup.py`, `requirements.txt`

### 지원 리소스

* Windows SDK
* Visual Studio 설치 인스턴스
* MSBuild
* .NET SDK
* 프로젝트별 `node_modules`
* 프로젝트별 `bin`, `obj`, `build`, `dist`, `.next`, `out`
* 프로젝트 경로와 드라이브
* 프로젝트별 manifest·lockfile
* 프로젝트별 Python 가상환경(`.venv`/`venv`/`env`)과 `__pycache__` 등 캐시
* conda 환경 (전역 named 환경은 정보 제공용, 자동 정리 대상 아님 — §19.5)

### 지원 질문

* 어떤 항목이 가장 많은 공간을 차지하는가?
* 특정 경로는 무엇인가?
* 특정 SDK를 어떤 프로젝트가 사용하는가?
* 이 항목을 없애면 어떤 프로젝트가 영향을 받는가?
* 안전하게 확보 가능한 공간은 얼마나 되는가?
* 이 프로젝트에서 다시 생성 가능한 항목은 무엇인가?

## 5.2 P1: 핵심 기능 완성 후 추가

* `global.json` 기반 .NET SDK 선택 분석
* `Directory.Build.props` 상위 경로 분석
* MSBuild `-preprocess` 실행
* MSBuild binary log 분석
* npm·pnpm 전역 저장소 탐지 (analysis-only 구현됨, 공식 cleanup 명령 안내)
* Android SDK·Gradle, Cargo, Maven 전역 저장소 탐지 (analysis-only 구현됨, 공식 cleanup 절차 안내)
* Docker 디스크 사용량 탐지 (`docker system df`, read-only 구현됨)
* HTML 또는 Markdown 리포트
* 삭제 전 프로젝트 빌드 검증 명령 생성

MSBuild의 preprocess 옵션은 import된 프로젝트 파일과 설정을 펼쳐 최종 구성을 확인하는 데 사용할 수 있고, binary log는 상세한 빌드 과정을 기록한다. 따라서 단순 XML 검색보다 높은 신뢰도의 근거로 활용할 수 있다.

## 5.3 P2: 여력이 있을 때 구현할 데몬

* 지정한 프로젝트 루트 변경 감시
* 변경된 디렉터리만 다시 스캔
* 파일 생성·삭제·크기 변경 이벤트 기록
* 마지막 관찰 시각 업데이트
* CLI의 반복 조회 속도 개선
* 최근 저장공간 증가 원인 표시

MVP 데몬은 모든 파일 접근을 추적하지 않는다.

```text
가능:
- 파일이 생성됨
- 파일이 삭제됨
- 크기가 증가함
- 프로젝트 디렉터리가 변경됨

불가능:
- 이 파일이 정확히 어떤 소스에서 생성됐는가
- 모든 읽기 파일이 필수 의존성인가
- 어떤 파일을 제거해도 빌드 결과가 완전히 동일한가
```

NTFS USN Change Journal은 볼륨의 파일 변경을 지속적으로 기록할 수 있지만, 변경의 사실과 이유를 중심으로 기록하며 오래된 레코드는 공간 확보를 위해 제거될 수 있다. 따라서 향후 증분 인덱스에는 유용하지만 의미적 의존성의 단독 근거로 사용해서는 안 된다.

---

# 6. 기능 명세

## F-01. 초기 설정

### 명령

```bash
libra init
```

### 동작

* 설정 파일 생성
* 스캔할 프로젝트 루트 입력
* 제외 경로 설정
* 데이터베이스 위치 설정
* 위험한 시스템 경로 자동 제외

### 설정 예시

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
  - AppData\Local\Temp
  - Windows
  - System Volume Information

scan:
  follow_reparse_points: false
  max_depth: 20

cleanup:
  default_mode: dry-run
  quarantine_days: 7
```

`libra init`은 생성물·vendored 디렉터리가 project로 오탐되거나 불필요하게
순회되지 않도록 위 기본 exclude를 설정합니다. `exclude`를 직접 작성하면
기본값을 대체하므로 유지할 기본 항목도 함께 적어야 합니다.

### 완료 조건

* 설정 파일 없이 실행하면 초기 설정 안내가 출력된다.
* 존재하지 않는 경로는 경고하되 다른 경로는 계속 분석한다.
* junction과 symbolic link는 기본적으로 따라가지 않는다.

---

## F-02. 초기 전체 스캔

### 명령

```bash
libra scan
libra scan --root D:\Projects
libra scan --full
```

### 동작

1. 설정된 프로젝트 루트 스캔
2. 프로젝트 manifest 탐색
3. 알려진 개발 도구 경로 탐색
4. 디렉터리 용량 계산
5. 프로젝트와 리소스 저장
6. 의존성 분석기 실행
7. 분석 결과 SQLite 저장

### 출력

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

### 구현 요구사항

* scan 진행률 표시
* 접근 권한 오류가 발생해도 전체 스캔 중단 금지
* 경로 정규화
* 동일 경로 중복 등록 방지
* reparse point 순환 방지
* 취소 신호 처리
* 오류 발생 경로 별도 기록

---

## F-03. 프로젝트 탐지

### 탐지 규칙

| 파일             | 프로젝트 종류                |
| -------------- | ---------------------- |
| `.sln`         | Visual Studio Solution |
| `.vcxproj`     | MSBuild C++            |
| `.csproj`      | MSBuild .NET           |
| `package.json` | Node.js                |
| `.git`         | Git 저장소                |

### 프로젝트 정보

```text
Project
- id
- name
- root_path
- drive
- project_type
- manifest_paths
- git_branch
- last_modified_at
- last_scan_at
- active_status
```

### 프로젝트 상태

* `ACTIVE`: 최근 수정되었거나 최근 사용 근거가 있음
* `STALE`: 일정 기간 수정되지 않음
* `ARCHIVED`: 사용자가 명시적으로 보관 지정
* `UNKNOWN`: 활동 판단 불가

MVP의 기본 stale 기준은 90일로 두되 설정 가능하게 한다.

---

## F-04. 개발 리소스 탐지

### Windows SDK

탐지 항목:

```text
Windows SDK
- version
- root path
- Include 디렉터리
- Lib 디렉터리
- Bin 디렉터리
- 총 논리 크기
- 설치 상태
```

### Visual Studio·MSBuild

Visual Studio 2017 이후에는 여러 인스턴스가 설치될 수 있으므로 단일 고정 경로에 의존하지 않는다. MVP에서는 Microsoft의 `vswhere.exe`를 우선 사용해 설치 인스턴스와 MSBuild 위치를 찾는다. `vswhere.exe`는 Visual Studio Installer에 포함되어 있으며 JSON 출력과 MSBuild 검색을 지원한다.

### .NET SDK

```text
dotnet --list-sdks
dotnet --list-runtimes
```

공식 .NET CLI가 설치된 SDK와 Runtime 목록을 제공하므로 직접 설치 폴더만 추측하지 않고 명령 결과를 우선 사용한다.

### Node 프로젝트 산출물

탐지 대상:

* `node_modules`
* `.next`
* `dist`
* `build`
* `out`
* 프로젝트 내부 `.cache`

### MSBuild 산출물

탐지 대상:

* `bin`
* `obj`
* `Debug`
* `Release`
* `x64`
* `x86`
* 프로젝트 설정에서 확인된 `OutputPath`
* 프로젝트 설정에서 확인된 `IntermediateOutputPath`

---

## F-05. 의존성 분석

### 핵심 그래프

```text
PROJECT REQUIRES RESOURCE
PROJECT OWNS ARTIFACT
PROJECT CAN_REGENERATE ARTIFACT
PROJECT DECLARES SDK_VERSION
RESOURCE SUPERSEDES RESOURCE
ARTIFACT GENERATED_BY TOOL
```

### Windows SDK 분석

다음 XML 항목을 검사한다.

```xml
<WindowsTargetPlatformVersion>
<WindowsTargetPlatformMinVersion>
<TargetPlatformVersion>
<PlatformToolset>
```

검사 위치:

* `.vcxproj`
* `.csproj`
* 프로젝트와 상위 경로의 `Directory.Build.props`
* 프로젝트가 직접 import하는 `.props`, `.targets`

### SDK 버전 해석

```text
10.0.22621.0
→ 정확한 버전 의존

10.0
→ 설치된 SDK 중 최신 버전을 선택할 가능성

빈 값
→ Visual Studio 또는 MSBuild 기본값 사용 가능

$(SomeProperty)
→ 추가 속성 해석 필요
```

단순 XML만으로 확정할 수 없는 항목은 `INFERRED` 또는 `UNKNOWN`으로 표시한다.

### Node 프로젝트 분석

* `package.json`
* `package-lock.json`
* `pnpm-lock.yaml`
* `yarn.lock`

`node_modules`는 lockfile과 manifest가 존재할 경우 재생성 가능한 프로젝트 리소스로 분류한다.

---

## F-06. 저장공간 요약

### 명령

```bash
libra summary
libra summary --drive C:
libra summary --type sdk
```

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

---

## F-07. 항목 설명

### 명령

```bash
libra explain windows-sdk:10.0.22621.0
libra explain "D:\Projects\OldWeb\node_modules"
```

### 필수 출력

* 항목 종류
* 경로
* 크기
* 생성 또는 설치 추정 시점
* 마지막 수정 시점
* 연결된 프로젝트
* 의존성 근거
* 재생성 가능 여부
* 삭제 영향
* 복구 방법
* 위험도
* 분석 신뢰도
* 분석하지 못한 범위

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

---

## F-08. 삭제 영향 분석

### 명령

```bash
libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"
libra impact windows-sdk:10.0.22621.0
```

### 영향 종류

* `RUN`: 이미 빌드된 프로그램 실행
* `BUILD`: 프로젝트 재빌드
* `DEBUG`: IDE 디버깅
* `RESTORE`: 의존성 다시 설치
* `CI`: 로컬에서 발견된 CI 설정
* `UNKNOWN`: 확인 불가

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

---

## F-09. 정리 계획 생성

### 명령

```bash
libra plan
libra plan --target 10GB
libra plan --risk safe
libra plan --project D:\Projects\OldWeb
```

### 분류 규칙

#### SAFE

* 프로젝트 내부에 존재
* 재생성 명령이 명확함
* manifest 또는 프로젝트 설정이 존재
* 사용자 원본 파일로 분류되지 않음
* 시스템 경로가 아님
* 활성 프로세스가 사용하는 경로가 아님

예:

* `node_modules`
* `bin`
* `obj`
* `dist`
* `.next`
* 명확한 빌드 출력 디렉터리

#### REVIEW

* 전역 캐시
* 참조 프로젝트가 없지만 다시 필요할 수 있는 SDK
* 장기간 사용되지 않은 프로젝트
* 재다운로드 비용이 큰 리소스
* 분석 신뢰도가 낮은 대용량 경로

pnpm의 공식 `store prune`도 현재 참조되지 않는 패키지를 제거하지만, 과거 브랜치로 전환하면 다시 다운로드가 필요할 수 있다고 설명한다. 따라서 전역 패키지 저장소는 무조건 `SAFE`가 아니라 `REVIEW`로 취급한다.

#### BLOCKED

* 현재 프로젝트가 명시적으로 참조
* 운영체제 또는 런타임 구성요소
* 사용자 데이터일 가능성이 있음
* 삭제 후 복구 방법이 불명확
* symbolic link 또는 junction 처리 불명
* 분석 범위 밖에 있는 의존 가능성 높음

### 계획 출력

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

---

## F-10. 안전한 격리

### 명령

```bash
libra clean --plan plan-20260717-001
libra clean --plan plan-20260717-001 --dry-run
```

### 원칙

* 기본값은 `--dry-run`
* `SAFE` 항목만 처리
* 시스템 리소스 처리 금지
* 삭제보다 같은 볼륨 내 격리 이동 우선
* 작업 단위 transaction 생성
* 성공한 항목만 DB 반영
* 일부 실패 시 전체 결과를 명확히 출력

### 격리 구조

```text
D:\Projects\.Libra-quarantine\
  tx-20260717-001\
    manifest.json
    OldWeb-node_modules\
    GameClient-build\
```

### transaction 정보

```text
Transaction
- id
- plan_id
- created_at
- original_path
- quarantine_path
- expected_size
- actual_size
- status
```

### 금지 조건

* 대상 경로가 스캔 루트 밖으로 정규화됨
* reparse point를 포함함
* 루트 디렉터리
* 드라이브 루트
* 시스템 보호 경로
* manifest와 실제 대상 경로가 불일치
* 대상이 스캔 이후 변경됨

---

## F-11. 복구

### 명령

```bash
libra restore tx-20260717-001
libra transactions
```

### 동작

* 원래 위치가 비어 있는지 확인
* 충돌 시 자동 덮어쓰기 금지
* 원래 위치로 이동
* DB 상태 변경
* 일부 복구 실패 시 상세 보고

---

## F-12. 결과 내보내기

### 명령

```bash
libra export --format json
libra export --format markdown
```

### 활용

* 발표 자료
* 버그 리포트
* 팀원 간 환경 공유
* 후속 GUI 개발
* AI 분석 입력

JSON은 그래프 전체가 아니라 사용자에게 필요한 요약 모델을 우선 제공한다.

---

# 7. CLI 명세

## 기본 명령 구조

```text
Libra
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
└─ daemon        # P2
```

## 공통 옵션

```bash
--config <path>
--json
--verbose
--no-color
--yes
--dry-run
```

## 종료 코드

| 코드 | 의미            |
| -: | ------------- |
|  0 | 성공            |
|  1 | 일반 오류         |
|  2 | 잘못된 입력        |
|  3 | 일부 경로 스캔 실패   |
|  4 | 권한 부족         |
|  5 | 안전 정책으로 작업 거부 |
|  6 | 복구 충돌         |
|  7 | 데이터베이스 오류     |

---

# 8. 위험도 및 신뢰도 모델

## 8.1 위험도 계산

위험도는 숫자 점수보다 규칙 기반 판정을 우선한다.

### BLOCKED 조건

하나라도 만족하면 `BLOCKED`다.

* 시스템 또는 런타임 구성요소
* 활성 프로젝트에서 명시적으로 사용
* 사용자 데이터 가능성
* 복구 방법 없음
* 대상 경로 불확실
* 현재 실행 중인 프로세스가 사용

### SAFE 조건

다음을 모두 만족해야 한다.

* 프로젝트 내부 산출물
* 알려진 생성 디렉터리
* 재생성 명령 존재
* 현재 원본 파일이 아님
* 시스템 경로가 아님
* 경로 정규화 및 링크 검증 통과
* 격리 가능한 크기와 권한

그 외는 `REVIEW`다.

## 8.2 신뢰도 계산

신뢰도는 `Classification`, `Ownership`, `Dependency`, `CleanupSafety`, `ScanCoverage`,
`Freshness` 축으로 나누고 가장 약한 축을 요약값으로 사용한다. 숫자는 통계적 확률이 아니라
**분석 범위 충족도**다.

`Freshness`는 마지막 관측 후 7일까지 100, 30일까지 80, 90일까지 50, 그 이후 20이다.
30일을 넘긴 `SAFE` 결과는 자동 정리 후보에서 제외하고 `EVIDENCE_STALE` 사유가 있는
`REVIEW`로 표시한다. 새 scan을 실행하면 최신성 근거가 갱신된다.

---

# 9. 기술 아키텍처

## 9.1 권장 기술 스택

### 언어: Go

선정 이유:

* 단일 실행 파일 배포
* CLI 개발 속도
* Windows 환경에서 비교적 간단한 빌드
* 병렬 파일 스캔 구현 용이
* 백그라운드 데몬으로 확장 가능
* 1주 프로젝트에서 Rust보다 보수적인 일정 수립 가능

### 주요 구성

* CLI: Cobra
* 출력: 표준 출력 formatter 또는 Lip Gloss
* 저장소: SQLite
* 설정: YAML
* 파일 감시: fsnotify 또는 Windows API wrapper
* XML: Go 표준 XML parser
* JSON: Go 표준 JSON parser
* 테스트: Go 기본 testing
* 빌드: GitHub Actions Windows runner

TUI는 이번 주 필수 범위에서 제외한다.

## 9.2 전체 구조

```text
CLI Commands
    │
    ▼
Application Services
    ├─ AnalysisOrchestrator
    ├─ ExplainService
    ├─ ImpactService
    ├─ PlanService
    └─ CleanupService
    │
    ▼
Domain Model
    ├─ Project
    ├─ Resource
    ├─ Dependency
    ├─ Evidence
    └─ CleanupTransaction
    │
    ▼
Adapters
    ├─ GenericFilesystemAdapter
    ├─ VisualStudioAdapter
    ├─ WindowsSdkAdapter
    ├─ MsBuildAdapter
    ├─ DotnetAdapter
    └─ NodeAdapter
    │
    ▼
SQLite Repository
```

## 9.3 Adapter 인터페이스

```go
type Adapter interface {
    Name() string
    Detect(ctx context.Context, env Environment) ([]Resource, error)
    Analyze(ctx context.Context, project Project, env Environment) ([]Evidence, error)
    Recommend(ctx context.Context, resource Resource) ([]Action, error)
}
```

Adapter는 파일을 직접 삭제하지 않는다.

삭제 가능성은 `CleanupService`가 전체 안전 정책을 검사한 뒤 결정한다.

---

# 10. 데이터 모델

## projects

```text
id
name
root_path
normalized_path
drive
project_type
last_modified_at
last_observed_at
status
scan_id
```

## resources

```text
id
resource_type
name
version
path
normalized_path
logical_size
reclaimable_size
regenerable
system_managed
last_modified_at
last_observed_at
risk
confidence
```

## dependencies

```text
id
source_type
source_id
target_type
target_id
relation
confidence
```

## evidence

```text
id
dependency_id
evidence_type
source_path
property_name
raw_value
resolved_value
collected_at
```

## scans

```text
id
started_at
finished_at
roots
file_count
error_count
status
```

## cleanup_plans

```text
id
created_at
target_bytes
selected_bytes
status
```

## cleanup_items

```text
id
plan_id
resource_id
expected_bytes
risk
action_type
reason
```

## transactions

```text
id
plan_id
started_at
finished_at
status
```

---

# 11. 저장공간 계산 정책

## 논리 크기

파일 경로를 따라 일반적인 파일 크기를 합산한다.

## 주의 대상

* hard link
* symbolic link
* NTFS junction
* sparse file
* 압축 파일
* OneDrive placeholder
* 권한이 없는 디렉터리

MVP에서는 다음 정책을 사용한다.

* reparse point는 따라가지 않는다.
* hard link의 정확한 물리 중복 제거는 P1로 미룬다.
* 화면에는 `논리 크기`라고 명확히 표시한다.
* 물리적 확보 예상치는 Adapter가 확신할 수 있을 때만 제공한다.
* 알 수 없으면 `최대 확보 가능 크기`로 표시한다.

---

# 12. 백그라운드 데몬 명세

## 12.1 데몬 MVP의 역할

```bash
libra daemon start
libra daemon status
libra daemon stop
libra events
```

### 처리 대상

* 설정된 프로젝트 루트
* 탐지된 대용량 산출물 디렉터리
* 선택한 패키지 캐시 경로

### 이벤트

```text
CREATE
DELETE
RENAME
SIZE_CHANGE
PROJECT_DISCOVERED
PROJECT_REMOVED
RESOURCE_DIRTY
```

### 처리 구조

```text
Filesystem event
    ↓
경로 정규화
    ↓
500ms~2s 단위 batch
    ↓
영향받은 디렉터리 dirty 처리
    ↓
해당 디렉터리만 다시 계산
    ↓
SQLite 갱신
```

## 12.2 데몬에서 하지 않을 것

* 모든 파일 read 추적
* 모든 프로세스의 I/O 추적
* ETW 기반 provenance 수집
* 파일마다 직접적인 의존성 edge 생성
* 실시간 자동 삭제

## 12.3 데몬 성공 기준

* 파일 추가 후 수 초 내 summary 반영
* 동일 파일에 대한 연속 이벤트를 batch 처리
* watcher 오류 시 전체 재스캔 필요 상태 표시
* 데몬 중단 중 발생한 변경은 다음 scan에서 복구
* 데몬이 없어도 모든 기본 CLI 기능 정상 동작

---

# 13. 안전한 삭제 정책

## 자동 처리 가능

* 프로젝트 `bin`
* 프로젝트 `obj`
* 프로젝트 `dist`
* 프로젝트 `build`
* 프로젝트 `.next`
* 프로젝트 `node_modules`

단, 탐지 규칙과 안전 검사를 모두 통과해야 한다.

## 계획만 제공

* Windows SDK
* Visual Studio 구성요소
* .NET SDK
* npm·pnpm 전역 캐시
* Docker 이미지와 빌드 캐시

## 절대 자동 처리 금지

* Docker Volume
* 데이터베이스
* 사용자 문서
* 소스 코드
* Git object
* `.env`
* 인증서·키
* 운영체제 디렉터리
* 알 수 없는 대용량 폴더

Docker는 공식적으로 사용하지 않는 이미지·컨테이너·네트워크·빌드 캐시를 정리하는 명령을 제공하지만, Volume은 기본 정리에 포함되지 않고 별도 옵션이 필요하다. 이처럼 각 생태계의 공식 정리 명령을 우선 활용하는 방향으로 확장한다.

현재 Libra는 `docker system df --format '{{json .}}'`를 읽어 Images, Containers,
Build Cache와 Local Volumes의 aggregate 용량을 `resources`/`summary`에 포함한다.
이미지·컨테이너·빌드 캐시는 `REVIEW`, Volume은 사용자 데이터 가능성 때문에
`BLOCKED`다. Libra는 `docker system prune`, `docker volume prune` 또는 remove 명령을
자동 실행하지 않는다.

---

# 14. 비기능 요구사항

## 성능

목표값이며 발표 전에 실제 장비로 측정한다.

* 프로젝트 루트 10만 파일 스캔 중 메모리 사용량 급증 방지
* 파일 스캔 worker 개수 제한
* SQLite batch insert
* 동일 경로 반복 `stat` 최소화
* 두 번째 스캔에서 변경 없는 디렉터리 재분석 최소화
* `summary`, `explain`, `impact`는 DB 조회 중심으로 즉시 응답

## 안정성

* 하나의 Adapter 실패가 전체 분석을 중단하지 않음
* Ctrl+C 처리
* DB transaction 사용
* cleanup 전 경로 재검증
* cleanup 후 실제 크기 재측정
* 접근 권한 오류 기록
* 로그에 민감한 파일 내용 저장 금지

## 개인정보 보호

* 모든 데이터는 로컬 저장
* 파일 본문 저장 금지
* `.env`, 인증서, 비밀키 내용 읽기 금지
* 기본 export에서 사용자 이름 경로 익명화 옵션 제공
* 원격 서버 전송 없음

---

# 15. 테스트 계획

## 15.1 단위 테스트

* 경로 정규화
* Windows 드라이브 대소문자 처리
* XML property 추출
* `Directory.Build.props` 탐색
* 프로젝트 타입 탐지
* 위험도 분류
* 신뢰도 계산
* 목표 용량 계획 알고리즘
* 시스템 경로 차단
* reparse point 차단

## 15.2 통합 테스트

임시 디렉터리에 다음 fixture를 생성한다.

```text
C-drive-fixture/
  WindowsKits/
    10.0.19041.0/
    10.0.22621.0/

D-drive-fixture/
  GameClient/
    GameClient.vcxproj
    build/
  OldWeb/
    package.json
    package-lock.json
    node_modules/
```

검증:

* GameClient와 특정 SDK 연결
* C 리소스와 D 프로젝트 연결
* node_modules 재생성 가능 판정
* target 용량 계획
* 격리 후 restore
* 권한 오류 처리

## 15.3 Golden output 테스트

CLI 출력이 예기치 않게 변하지 않도록 주요 출력 결과를 snapshot과 비교한다.

대상:

* `summary`
* `explain`
* `impact`
* `plan`
* `clean --dry-run`

## 15.4 실제 Windows 장비 테스트

최소 두 대가 이상적이다.

* Visual Studio가 설치된 장비
* Visual Studio가 없거나 설치 구성이 다른 장비
* C와 D 드라이브가 있는 장비
* Node 프로젝트가 여러 개 있는 장비

---

# 16. 완료 기준

## 필수 데모가 성공해야 하는 질문

### 질문 1

```text
Windows SDK 10.0.19041.0을 삭제해도 되는가?
```

도구가 반드시 보여줘야 하는 정보:

* SDK 크기
* SDK 경로
* 참조 프로젝트 수
* 프로젝트 경로
* 근거 파일
* 빌드 영향
* 실행 영향 추정
* 위험도
* 신뢰도
* 공식 제거 권장

### 질문 2

```text
C 드라이브의 이 리소스를 제거하면 D 드라이브 프로젝트가 영향을 받는가?
```

도구가 반드시 C 리소스에서 D 프로젝트로 연결되는 edge를 보여줘야 한다.

### 질문 3

```text
안전하게 5GB를 확보하려면 무엇을 정리해야 하는가?
```

도구가 다음을 제공해야 한다.

* SAFE 후보
* REVIEW 후보
* BLOCKED 후보
* 예상 확보 공간
* 재생성 방법
* dry-run
* 최소 하나의 격리 및 복구 성공

---

# 17. 3인 팀 역할 분담

## 팀원 A: Indexing & Platform

책임:

* 파일 스캐너
* 경로 정규화
* 크기 계산
* SQLite schema와 repository
* incremental scan
* 데몬 stretch

주요 산출물:

```text
AnalysisOrchestrator
FilesystemRepository
SQLiteRepository
Daemon
```

## 팀원 B: Dependency Analysis

책임:

* 프로젝트 탐지
* Windows SDK Adapter
* Visual Studio·MSBuild Adapter
* .NET Adapter
* Evidence와 graph 생성

주요 산출물:

```text
ProjectDetector
WindowsSdkAdapter
MsBuildAdapter
DotnetAdapter
```

## 팀원 C: CLI & Safety

책임:

* CLI 명령
* summary·explain·impact 출력
* plan 알고리즘
* cleanup·quarantine·restore
* Node Adapter
* 사용자 문서
* 통합 테스트와 데모 데이터

주요 산출물:

```text
CLI
PlanService
CleanupService
RestoreService
NodeAdapter
Demo fixtures
```

## 공통 책임

* 데이터 모델 변경은 3명 합의
* cleanup 관련 코드는 최소 2명 리뷰
* 매일 마지막 1시간 통합
* 각 기능은 fixture 테스트와 함께 merge
* 핵심 브랜치는 항상 실행 가능한 상태 유지

---

# 18. 일주일 개발 계획

## Day 1 — 범위 확정과 뼈대 구축

### 공통

* 제품 요구사항 확정
* P0·P1·P2 분리
* 데이터 모델 확정
* fixture 구조 확정
* Git branch와 PR 규칙 확정

### 팀원 A

* Go 프로젝트 초기화
* SQLite 연결
* schema migration
* config loader

### 팀원 B

* Project와 Resource domain 모델
* Adapter 인터페이스
* fixture용 `.vcxproj`, `.csproj`, `package.json` 작성

### 팀원 C

* Cobra CLI 구조
* `init`, `scan`, `summary` 빈 명령
* 출력 formatter
* README 초안

### 종료 기준

```text
libra --help
libra init
libra scan
```

세 명의 환경에서 실행된다.

---

## Day 2 — 파일 스캔과 프로젝트 탐지

### 팀원 A

* 병렬 디렉터리 스캐너
* 크기 계산
* 접근 권한 오류 처리
* reparse point 무시
* DB 저장

### 팀원 B

* `.sln`, `.vcxproj`, `.csproj`, `package.json`, `.git` 탐지
* 프로젝트 메타데이터 생성
* 상위 프로젝트 루트 결정

### 팀원 C

* scan 진행률
* scan 결과 출력
* `projects`, `resources` 명령
* scanner integration test

### 종료 기준

* 실제 `D:\Projects`에서 프로젝트 목록 출력
* 프로젝트별 `node_modules`, `bin`, `obj`, `build` 크기 출력
* 스캔 결과가 SQLite에 저장됨

---

## Day 3 — Windows 개발환경 Adapter

### 팀원 A

* 알려진 시스템 리소스 경로 스캔
* 리소스 크기 집계
* Windows SDK 버전별 grouping

### 팀원 B

* `vswhere.exe` 연동
* Visual Studio·MSBuild 위치 탐지
* Windows SDK Adapter
* .NET SDK 목록 탐지
* `.vcxproj` Windows SDK 속성 파싱

### 팀원 C

* `resources --type windows-sdk`
* `explain` 기본 출력
* 실제 PC fixture 수집
* Adapter 실패 UI

### 종료 기준

다음 명령이 실제 결과를 보여준다.

```bash
libra resources --type windows-sdk
libra explain windows-sdk:<version>
```

---

## Day 4 — 의존성 그래프와 Impact

### 팀원 A

* dependency·evidence DB 구현
* graph query
* C와 D 드라이브 간 edge 지원

### 팀원 B

* 프로젝트 → Windows SDK 연결
* 프로젝트 → .NET SDK 연결
* 프로젝트 → node_modules·build output 연결
* `Directory.Build.props` 기본 탐색

### 팀원 C

* `impact` 명령
* RUN·BUILD·DEBUG·RESTORE 출력
* 위험도와 신뢰도 formatter
* Golden output test

### 종료 기준

프로젝트의 핵심 데모가 read-only 상태로 완성된다.

```bash
libra impact windows-sdk:10.0.xxxxx.0
```

C 드라이브 SDK와 D 드라이브 프로젝트 관계가 출력되어야 한다.

---

## Day 5 — 정리 계획과 격리

> 현재 상태: plan snapshot, 실행 직전 경로·타입·크기·mtime·소유권·SAFE·Git·reparse·보호 경로 재검증, volume별 manifest-first quarantine, 부분 실패 transaction, 충돌 없는 restore까지 구현됐다. 영구 삭제는 제공하지 않는다.

### 팀원 A

* 디렉터리 변경 시 크기 재확인
* transaction repository
* DB transaction 처리

### 팀원 B

* 재생성 가능성 규칙
* Node와 MSBuild 산출물 분류
* 공식 복구 명령 생성

### 팀원 C

* `plan --target`
* `clean` dry-run 및 `--execute --yes`
* 같은 volume quarantine와 schema-versioned manifest
* `transactions`와 충돌 없는 `restore`
* 시스템 경로 denylist

### 종료 기준

다음 시나리오가 성공한다.

```bash
libra plan --target 1GB
libra clean --plan <id>
libra clean --plan <id> --execute --yes
libra transactions
libra restore --transaction <transaction-id>
```

시스템 SDK는 정리 계획에 포함되더라도 직접 삭제되지 않아야 한다.

---

## Day 6 — 통합, 테스트, 성능 측정

### 공통 우선순위

1. 실제 Windows 장비에서 end-to-end 테스트
2. 경로 및 권한 오류 수정
3. 삭제 안전성 검토
4. 성능 병목 개선
5. CLI 문구 정리

### 팀원 A

* 큰 fixture 성능 테스트
* DB batch 처리
* scanner cancellation
* 로그 정리

### 팀원 B

* 분석 오탐 사례 수정
* unresolved property 처리
* 분석 범위와 warning 출력

### 팀원 C

* 설치·실행 스크립트
* 데모 fixture 자동 생성
* README 사용법
* 발표 시나리오

### 종료 기준

* 새 Windows 장비에서 README만 보고 실행 가능
* 핵심 세 질문에 답할 수 있음
* cleanup 및 restore 통합 테스트 통과
* 치명적 crash 없음

---

## Day 7 — 발표 완성 및 Stretch

### 오전: 필수 기능 고정

* 새로운 P0 기능 추가 금지
* 버그 수정만 진행
* 데모 데이터 고정
* 실행 파일 release build
* 백업 데모 영상 녹화

### 오후: 여유가 있는 경우만

#### 우선순위 1

* `Libra daemon start`
* 프로젝트 루트 변경 감시
* dirty directory 증분 재계산

#### 우선순위 2

* MSBuild preprocess
* Markdown report
* pnpm store 분석

#### 우선순위 3

* Docker `system df` Adapter
* 간단한 interactive 선택 UI

### 종료 기준

* release binary
* 최종 README
* 아키텍처 다이어그램
* 데모 스크립트
* 테스트 결과
* 알려진 한계 목록
* 발표용 backup fixture

---

# 19. 일정 지연 시 기능 컷 순서

일정이 밀리면 다음 순서로 제거한다.

1. 백그라운드 데몬
2. Docker Adapter
3. pnpm 전역 저장소
4. Markdown·HTML report
5. MSBuild binary log 분석
6. 실제 build 검증
7. `.csproj`의 복잡한 조건식 해석
8. quarantine UI 개선

끝까지 유지할 기능:

* scan
* 프로젝트 탐지
* Windows SDK 탐지
* 프로젝트와 SDK 연결
* summary
* explain
* impact
* plan
* dry-run
* 최소한의 격리와 복구

---

# 20. AI 활용 원칙

## AI에게 맡기기 좋은 작업

* Adapter boilerplate
* XML·JSON fixture 생성
* 단위 테스트 초안
* CLI help 문구
* SQLite query 초안
* 에러 케이스 목록
* README와 발표 자료 정리
* 반복되는 formatter 작성

## 반드시 사람이 검토할 작업

* 경로 정규화
* symbolic link와 junction 처리
* recursive directory 처리
* 삭제·이동·복구 코드
* 시스템 경로 denylist
* 위험도 분류 규칙
* 관리자 권한 동작
* 실제 SDK 영향 판단
* 외부 명령 실행과 인자 escaping

## 개발 규칙

* AI가 작성한 cleanup 코드는 테스트 없이 merge 금지
* 삭제 코드는 한 번에 한 명만 수정
* core schema를 AI가 임의 변경하지 않도록 문서 고정
* AI에게 전체 코드베이스를 동시에 대규모 수정시키지 않음
* 한 PR은 하나의 책임만 가짐
* 각 Adapter는 fixture를 포함해야 함

---

# 21. 핵심 리스크와 대응

## 리스크 1: 스캔이 너무 느림

대응:

* 전체 C 드라이브를 무조건 재귀 탐색하지 않음
* 설정된 프로젝트 루트와 알려진 리소스 경로 우선
* worker 수 제한
* 디렉터리별 batch 저장
* 변경 없는 경로 재사용

## 리스크 2: SDK 분석 오탐

대응:

* `DECLARED`, `RESOLVED`, `INFERRED` 구분
* 확정할 수 없는 경우 `REVIEW`
* 분석하지 않은 브랜치와 드라이브 표시
* 시스템 SDK 자동 삭제 금지

## 리스크 3: 데이터 손실

대응:

* 기본 dry-run
* 격리 우선
* transaction 기록
* 시스템 경로 denylist
* cleanup 전 경로 재검증
* 최소 2명 코드 리뷰

## 리스크 4: 기능 범위 폭발

대응:

* Windows 전용
* Adapter 3개 이내
* 모든 언어 지원 금지
* 데몬은 stretch
* 프로세스 I/O 추적 금지
* GUI 금지

## 리스크 5: 발표 환경에서 실제 정리 대상이 없음

대응:

* 실제 환경 read-only 데모
* 별도의 대용량 fixture 제공
* C·D 드라이브 관계를 흉내 내는 테스트 fixture
* 미리 생성한 DB snapshot 준비
* backup 데모 영상 준비

---

# 22. 발표 데모 시나리오

## 1단계: 현황 스캔

```bash
libra scan
libra summary
```

보여줄 내용:

* 프로젝트 개수
* 개발 리소스별 용량
* 확보 가능한 공간
* C·D 드라이브 분포

## 2단계: Windows SDK 질문

```bash
libra resources --type windows-sdk
libra explain windows-sdk:10.0.22621.0
```

보여줄 내용:

* 어떤 프로젝트가 사용하는지
* D 드라이브 프로젝트와의 관계
* 선언 근거
* 삭제 영향

## 3단계: 목표 용량 정리

```bash
libra plan --target 5GB
```

보여줄 내용:

* SAFE·REVIEW·BLOCKED
* 사용자 데이터가 자동 제외됨
* SDK가 자동 삭제 대상이 아님

## 4단계: 안전한 격리

```bash
libra clean --plan <id>
libra summary
```

보여줄 내용:

* 실제 공간 감소
* transaction ID
* 원본 파일이 격리됨

## 5단계: 복구

```bash
libra restore <transaction-id>
```

보여줄 내용:

* 원래 위치 복원
* 프로젝트 상태 회복
* 안전한 관리 과정

## 6단계: 데몬이 완성된 경우

```bash
libra daemon start
```

빌드 산출물을 생성한 뒤:

```bash
libra events
libra summary
```

최근 증가한 저장공간이 자동 반영되는 모습을 보여준다.

---

# 23. 향후 발전 방향

## Phase 2

* NTFS USN Journal 기반 증분 인덱스
* MSBuild binary log 자동 수집
* `Libra trace -- <build command>`
* pnpm·npm 전역 캐시 Adapter — 구현 완료 (read-only)
* Docker Adapter
* Git의 다른 로컬 브랜치 분석
* SDK 제거 전 자동 빌드 검증

## Phase 3

* ETW 기반 개발 프로세스 관찰
* 프로젝트별 저장공간 증가 타임라인
* Android SDK·Gradle Adapter — 구현 완료 (read-only)
* Cargo·Maven Adapter — 구현 완료 (read-only)
* 로컬 웹 대시보드
* GitHub Issue 또는 리포트 내보내기

## 장기 비전

> **개발환경의 프로젝트와 공유 리소스를 추적하고, 더 이상 도달할 수 없는 개발 산출물을 식별하는 의미 기반 로컬 Garbage Collector**

---

# 24. 최종 제품 정의

Libra의 핵심은 디스크에서 큰 파일을 찾는 것이 아니다.

```text
기존 도구:
이 폴더는 3.1GB입니다.

Libra:
이 폴더는 Windows SDK 10.0.22621.0이며 3.1GB입니다.
D:\Projects\GameClient가 이 버전을 명시적으로 사용합니다.
삭제해도 기존 실행 파일은 동작할 가능성이 높지만,
프로젝트 재빌드는 실패할 것으로 예상됩니다.
따라서 자동 정리 대상에서 제외했습니다.
```

1주 프로젝트의 성공 기준은 많은 생태계를 지원하는 것이 아니라, 다음 두 질문에 신뢰할 수 있는 근거와 함께 답하는 것이다.

> **“이것은 왜 내 컴퓨터에 존재하는가?”**

> **“이것을 없애면 무엇이 영향을 받는가?”**
