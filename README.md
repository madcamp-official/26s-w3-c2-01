# 26s-w3-c2-01

## 팀원

| 이름 | 학교 | GitHub | 역할 |
|---|---|---|---|
|양우현|KAIST|hyun020215|  |
|임유빈|서울대학교|lunar-yoobin|  |
|최재윤|KAIST|Jaeyun-18|  |

## 선택 옵션

- [x] 옵션 1. Build the Core
- [ ] 옵션 2. Launch & Spread

---

## 기획안

- **산출물 주제:** 로컬 개발 프로젝트와 SDK·도구·캐시·빌드 산출물 사이의 의존 관계를 분석해, 무엇이 디스크 공간을 차지하고 있고 지우면 무엇이 깨지는지 근거와 함께 설명하는 Windows용 CLI 도구 `libra`.
- **문제의식:** 기존 저장공간 분석기는 "이 폴더가 크다"까지만 보여줄 뿐, "이 SDK를 어떤 프로젝트가 쓰는가", "지우면 어떤 프로젝트가 영향을 받는가", "이건 원본인가 재생성 가능한 산출물인가"에는 답하지 못한다. `libra`는 파일 크기 대신 **프로젝트 ↔ 리소스 관계와 판단 근거**를 우선 제공한다.
- **핵심 원칙:** 기본은 항상 read-only(scan/summary/explain/impact/plan은 파일을 건드리지 않음), 시스템 구성요소(Windows SDK, Visual Studio, `C:\Windows` 등)는 절대 직접 삭제하지 않고 계획·근거만 제공, 모든 의존 관계에는 근거 종류(DECLARED/RESOLVED/OBSERVED/INFERRED/UNKNOWN)를 함께 표시.
- **자세한 기능 명세와 개발 일정 원본:** `docs/libra_cli_commands_and_schedule.md`, 팀 협업 규칙은 `docs/libra_collaboration_rules.md`, cross-team 계약(스키마·JSON envelope·종료 코드 등 "권위 있는" 기준 문서)은 `docs/libra_integration_contracts.md`.

### 지금까지 진행한 일정 (git 커밋 기준)

| 날짜 | 진행 내용 |
|---|---|
| 2026-07-17 (Day 1) | 저장소 초기화, 기획서 초안, Cobra 기반 CLI 골격과 `init/scan/summary/explain/impact` 명령 뼈대, SQLite 연결과 초기 schema migration, YAML 설정 로더, domain 모델·adapter 인터페이스 정의, 팀 협업 규칙 문서, Windows CI 구성 |
| 2026-07-18 (Day 2~4) | 경계 검증이 있는 병렬 파일 스캐너, 경로 정규화, cross-team 통합 계약 문서화, Windows SDK·.NET SDK·Visual Studio(vswhere) 리소스 탐지와 시스템 경로 차단(safety), Node 프로젝트·산출물 탐지, 프로젝트/리소스/의존성 그래프(evidence 포함)를 SQLite에 저장, `scan`이 실제 분석 파이프라인(AnalysisOrchestrator)에 연결되어 `projects`/`summary`가 실제 데이터를 조회하도록 완성, golden test 도입 |
| 2026-07-19 | `resources`/`explain`/`impact` 명령 구현 완료, README·명령어 상태표 동기화 |
| 2026-07-20 (Day 4 리뷰 · Day 5) | Day 4 코드 리뷰(협업/계약/구조적 이슈 정리), cleanup evidence·위험도 정책(risk policy) 도입, cleanup plan snapshot 저장, `plan --target`/`clean`(dry-run) 구현, 같은 볼륨 quarantine·복구 transaction(`clean --execute`, `restore`) 완성 |
| 2026-07-21 (Day 5, 오늘까지) | 버그 수정(node_modules 프로젝트 오탐, 프로젝트 크기 0B 오표시, scan 경고 노출 개선), `export`/`purge`/`daemon`/`events` 명령 추가, Docker·Android·Gradle·Cargo·Maven·npm·pnpm·Conda 생태계 어댑터(analysis-only) 추가, 6축 신뢰도(confidence profile)와 구조화된 risk reason 도입, 전역 `--json` envelope와 종료 코드 계약 확정, `init` 없이는 다른 명령을 실행할 수 없도록 하는 전역 가드(`requireInit`) 도입, `scan` 실행 중 실시간 진행률 바(progress bar) 표시 추가, macOS 전용 개발 캐시 어댑터 5종(Xcode DerivedData/CocoaPods/SwiftPM/Homebrew/iOS Simulator, analysis-only) 추가, macOS 시스템 경로(`/System`/`/Library`/`/usr` 등) 보호 분류 추가, 실제 별도 APFS 볼륨과 권한 오류(chmod 000) 시나리오로 clean/restore 안전성 검증 |
| 2026-07-22 | `.NET SDK` 탐지를 macOS/Linux까지 확장(`dotnet` CLI는 원래 크로스플랫폼이었음), Xcode(`.xcodeproj`)·Xcode Workspace(`.xcworkspace`)·SwiftPM(`Package.swift`) 프로젝트 탐지 추가, 프로젝트 소유 `Pods`/`.build` 산출물을 `node_modules`와 동일한 SAFE 경로로 연결, 활성 Xcode 자체를 `xcode-install` 시스템 리소스로 탐지, `.xcodeproj` 프로젝트 → 활성 Xcode REQUIRES 의존성 분석기 추가(macOS 프로젝트도 Windows SDK/MSBuild와 동급으로 `explain`/`impact`의 의존성·삭제 영향 분석 대상이 됨; SwiftPM은 어떤 Swift toolchain으로도 빌드 가능해 Xcode 의존으로 보지 않음), 실제 NTFS 볼륨(junction/reparse point, `icacls` ACL 거부, 다른 프로세스가 잠근 파일, DB-파일시스템 불일치)을 대상으로 한 Windows 전용 e2e 테스트 추가로 `docs/libra_integration_contracts.md`의 "Windows 실제 volume junction/ACL 통합 테스트" 오픈 항목을 대부분 구현 완료 처리(hidden attribute 단독 케이스는 아직 미포함), 실제 `WINDIR`/`ProgramFiles`/`ProgramFiles(x86)` 환경변수 기반 시스템 경로 negative fixture로 `C:\Windows`·Program Files·Windows SDK·Visual Studio·.NET Runtime·Docker Volume이 항상 `BLOCKED`임을 검증 |

---

## 기능 명세서

`libra`는 Windows 10/11의 NTFS 로컬 드라이브를 대상으로 하며 관리자 권한 없이 실행된다. macOS/Linux에서도 빌드와 테스트는 CI로 검증되지만(`internal/adapter/platform.go`의 `RequireWindows`), Windows SDK·Visual Studio·MSBuild(vswhere) 같은 OS 전용 리소스 탐지기는 그 플랫폼에서 실행 시 "unsupported platform" 오류로 명시적으로 미지원을 알린다.

### 프로젝트 탐지

| 대상 | 지원 여부 |
|---|---|
| Git 저장소 (`.git`) | ✅ 다른 언어·빌드 매니페스트가 같은 root에 없을 때만 fallback 프로젝트로 탐지 |
| Node.js (`package.json`) — npm/Yarn/pnpm workspace 관계 포함, `node_modules` 내부를 프로젝트로 오탐하지 않도록 경계 처리됨 | ✅ |
| MSBuild C++ (`.vcxproj`) / .NET (`.csproj`) — XML에서 `WindowsTargetPlatformVersion` 등 속성 파싱 | ✅ |
| Python (`pyproject.toml`/`Pipfile`/`setup.py`/`requirements.txt`) | ✅ |
| Visual Studio Solution (`.sln`) | ⚠️ Workspace로만 취급되어 소속 프로젝트를 묶어줄 뿐, 그 자체가 독립된 분석 대상(BuildProject)은 아님 |
| Java/Android (`pom.xml`, `build.gradle[.kts]`), Go (`go.mod`), Rust (`Cargo.toml`) | ✅ 매니페스트 기반 프로젝트 단위 탐지 (Android Gradle 플러그인 표식 구분) |
| Cargo `target/`, Maven `target/`, Gradle `build/` | ✅ 프로젝트 소유 산출물 분석·격리 cleanup·restore (`Cargo.lock` 및 manifest/경로/Git/reparse 안전 gate 적용) |
| Xcode (`.xcodeproj`), Xcode Workspace (`.xcworkspace`, 최상위 `<FileRef>` 멤버만) (macOS) | ✅ |
| Swift Package (`Package.swift`, `// swift-tools-version:` 선언 파싱) (macOS) | ✅ |

### 리소스 탐지 (scan에 실제로 연결되어 있음)

| 리소스 | 지원 여부 / 방식 |
|---|---|
| Windows SDK | ✅ 파일시스템 기반 탐지, 버전별 그룹핑 |
| Visual Studio 설치본 · MSBuild | ✅ `vswhere.exe` 연동으로 설치 인스턴스와 MSBuild 위치 탐지 |
| .NET SDK / .NET Runtime | ✅ `dotnet --list-sdks` / `--list-runtimes` 결과 기반. 크로스플랫폼 CLI라 macOS/Linux도 지원(`exec.LookPath`), Windows만 고정 설치 경로를 그대로 사용 |
| Node `node_modules`, 빌드 산출물(`bin`/`obj`/`build`/`dist`/`.next`/`out`/`Debug`/`Release`, Python `__pycache__`/`.pytest_cache`/`.mypy_cache` 포함) | ✅ |
| Docker 리소스 사용량 | ✅ `docker system df --format '{{json .}}'` 결과를 읽어 Images/Containers/Build Cache/Volumes 집계 (read-only) |
| Android SDK, Gradle 전역 캐시, Cargo 레지스트리/git 전역 캐시, Maven 로컬 저장소, npm 캐시, pnpm store | ✅ analysis-only — 프로젝트 산출물과 달리 탐지·크기 집계만 하고 공식 정리 명령을 안내(직접 정리 실행은 하지 않음) |
| Xcode DerivedData, CocoaPods 캐시, SwiftPM 캐시, Homebrew 캐시, iOS Simulator 캐시 (macOS) | ✅ analysis-only — 위와 동일하게 탐지·크기 집계·공식 정리 명령 안내만 수행 |
| iOS Simulator `Devices/`(기기별 설치 앱·데이터, 보통 가장 큰 소비처) (macOS) | ✅ analysis-only — seed된 상태를 담을 수 있어 Docker Volume처럼 항상 `REVIEW`, 격리/숙청 대상 아님, `xcrun simctl delete unavailable` 안내. runtime 이미지는 시스템 구성요소라 제외 |
| Conda 환경 | ✅ 전역 named 환경은 정보 제공용(REQUIRES 관계), 프로젝트 내부 prefix 환경은 OWNS 관계로 구분 |
| 활성 Xcode 설치(`xcode-select`/`xcodebuild -version`), 프로젝트 소유 `Pods`/`.build` (macOS) | ✅ Xcode는 Visual Studio와 같은 급의 시스템 리소스(`BLOCKED`, 현재 활성 설치본만 — 비활성 Xcode는 미탐지), `Pods`/`.build`는 각각 `Podfile.lock`/`Package.resolved` 있으면 `node_modules`처럼 `SAFE` 후보 |
| Docker Volume | ⚠️ 탐지는 되지만 사용자 데이터일 수 있어 항상 `BLOCKED`, 자동/수동 삭제 명령 모두 미제공 |
| apt 등 macOS 외 패키지 매니저, 커널 드라이버/파일시스템 minifilter | ❌ 미지원 |
| 완전한 block-level 중복 파일(dedup) 탐지 | ❌ 미지원 |

### 의존성 분석

- **지원:** MSBuild 프로젝트 → Windows SDK/.NET SDK (XML property 기반, 근거를 `DECLARED`/`RESOLVED`/`INFERRED`/`UNKNOWN`으로 구분), Node 프로젝트 → `node_modules`/빌드 산출물(lockfile 존재 여부로 재생성 가능 여부 판단), Conda 의존성 분석기, `.xcodeproj` 프로젝트 → 활성 Xcode(`INFERRED`; 빌드에 전체 Xcode가 필요하므로). SwiftPM은 어떤 Swift toolchain으로도 빌드되므로 Xcode에 의존한다고 보지 않고, toolchain 관계를 `UnverifiedScope`로만 기록한다(거짓 삭제-영향 방지).
- **미지원:** `Directory.Build.props` 상위 경로의 완전한 해석, MSBuild `-preprocess`/binary log 분석, `Condition=` 속성부 완전 평가(대신 `UnverifiedScope`에 "평가하지 못한 범위"로 별도 기록됨).

### 위험도 분류 (`internal/app/risk_policy.go`)

| 조건 | 결과 |
|---|---|
| Android SDK | `BLOCKED` — `sdkmanager --uninstall` 또는 Android Studio SDK Manager 안내 |
| Xcode 설치 자체 | `BLOCKED` — App Store 또는 Apple Developer 다운로드 페이지에서 재설치 안내 |
| 전역 패키지 캐시(npm/pnpm/Maven/Cargo/Gradle/Xcode DerivedData/CocoaPods/SwiftPM/Homebrew/iOS Simulator) | `REVIEW` — 공식 정리 명령 안내 |
| macOS 시스템 경로(`/System`/`/Library`/`/usr`/`/bin`/`/sbin`/`/Applications`) | `BLOCKED` — Windows의 `WINDIR`/`ProgramFiles` 보호와 동일한 역할, `~/Library`는 제외(그 아래에서 위 macOS 캐시들을 탐지하므로) |
| Docker Volume | `BLOCKED` — 사용자 데이터 가능성 |
| Docker 캐시(이미지/컨테이너/빌드 캐시) | `REVIEW` — Docker 공식 명령으로 정리 필요 |
| 시스템 관리 경로 또는 `system_managed` 리소스 | `BLOCKED` |
| 현재 스캔에서 어떤 프로젝트든 실제로 참조 중 | `BLOCKED` |
| 판단에 결정적인 미확인 사항(critical unknown)이 있음 | `REVIEW` |
| 재생성 가능하고, 프로젝트 소유·알려진 산출물 경로·reparse point 없음·Git 추적 원본 부재가 모두 검증됨 | `SAFE` |
| 그 외 전부 | `REVIEW` |

### 신뢰도 모델 (6축, `internal/domain/risk_assessment.go` `ConfidenceProfile`)

`Classification`/`Ownership`/`Dependency`/`CleanupSafety`/`ScanCoverage`/`Freshness` 여섯 축을 0~100으로 따로 매기고, 전체 신뢰도는 **가장 약한 축의 값**을 사용한다(통계적 확률이 아니라 분석 범위 충족도). `Freshness`는 마지막 관측 후 7일 이내 100, 30일 80, 90일 50, 그 이후 20이며, `SAFE`로 분류된 항목이라도 `Freshness < 80`이면 `EVIDENCE_STALE` 사유와 함께 자동으로 `REVIEW`로 강등된다.

### 정리 파이프라인 (plan → clean → restore / purge)

- `plan`은 `SAFE` 후보만 신뢰도 → 크기 순으로 목표 용량까지 자동 선택한다. `REVIEW`/`BLOCKED`는 절대 자동 선택되지 않고 표시만 된다.
- `clean`은 기본이 dry-run이며, 실행(`--execute`) 시에도 삭제가 아니라 같은 볼륨의 격리 디렉터리로 이동만 한다.
- `restore`는 원본 위치가 비어 있을 때만 복구하며 기존 파일을 덮어쓰지 않는다.
- `purge`는 격리 후 지정한 보존 기간(`quarantine_days`, 기본 7일)이 지난 항목만 대상이며 기본도 dry-run이다. **`purge --execute`만이 되돌릴 수 없는 유일한 명령이다.**
- **미지원:** 시스템 구성요소(Windows SDK, Visual Studio 등) 직접 삭제, AI가 판단해서 자동으로 삭제하는 기능, 삭제 전 자동 빌드 검증.
- **실제 파일시스템 검증(Windows):** `internal/safety/quarantine_windows_edgecases_test.go`·`internal/safety/cleanup_validator_windows_test.go`·`internal/app/cleanup_service_windows_edgecases_test.go`가 실제 NTFS junction(`mklink`), `icacls`로 접근 거부된 경로, 다른 프로세스가 배타적으로 잠근 파일, 격리 항목이 사라진 뒤의 DB-파일시스템 불일치, 여러 항목 중 일부만 실패하는 partial quarantine을 실제 I/O로 재현해 `CleanupValidator`/`QuarantineEngine`/`CleanupService.Execute`가 안전하게 실패·롤업하는지 검증한다(`//go:build windows`). hidden attribute 단독 케이스는 아직 다루지 않는다.

### 백그라운드 데몬 (`daemon` / `events`)

- **지원:** `daemon start/status/stop`이 설정된 project root들을 2초 간격으로 폴링해 파일 인벤토리를 비교한다. 변경된 root만 `scan --root`로 증분 갱신하며 `.libra-events.jsonl`에 `CREATE`/`DELETE`/`RENAME`/`SIZE_CHANGE`/`MODIFY`와 `INCREMENTAL_SCAN`을 경로·크기와 함께 기록한다.
- **미지원:** 변경 디렉터리 내부만 재계산하는 하위 디렉터리 단위 병합, ETW 기반 파일 접근 추적, 데몬에 의한 자동 정리(관찰과 재스캔만 수행).

### 출력 계약

전역 `--json` 플래그는 `init`/`config show·validate·set`/`scan`/`summary`/`projects`/`resources`/`issues`/`explain`/`impact`/`plan`/`clean`/`restore`/`transactions`/`export`/`purge`/`daemon start·status·stop`/`events`를 포함한 모든 결과 명령에 다음과 같은 공통 envelope를 적용한다(schema version 1).

```json
{
  "command": "summary",
  "schema_version": 1,
  "outcome": "SUCCESS",
  "data": { "...": "명령별 결과" },
  "issues": [],
  "unverified": []
}
```

`outcome`은 `SUCCESS`/`PARTIAL`(일부만 처리됨, 예: scan 도중 일부 경로 권한 오류)/`FAILED`(자체 작업은 실패했지만 이유는 함께 출력, 예: 정리 대상이 전혀 없어 실패한 transaction) 중 하나다. `export --format json`은 이 envelope와는 별개인 이식 가능한 리포트 포맷이며, envelope로 감싼 export 결과가 필요하면 `libra --json export`를 쓴다.

### 종료 코드

| 코드 | 의미 |
|-: | --- |
|  0 | 성공 또는 dry-run 성공 |
|  1 | 인자·일반 명령 오류 |
|  2 | target/plan/transaction 대상을 찾을 수 없음 |
|  3 | DB·파일시스템 내부 오류 |
|  4 | safety 정책에 의해 차단됨 |
|  5 | 부분적으로만 성공한 clean/restore/purge |
|130 | 사용자 취소(확인 프롬프트에서 `y` 이외 입력) |

### 그 외 명시적으로 다루지 않는 범위

Xcode DerivedData의 프로젝트별 소유권 연결(현재는 §macOS 캐시처럼 전역 집계만, `info.plist`의 `WorkspacePath` 기반 프로젝트별 분리는 이중 계산 위험 때문에 보류), iOS Simulator runtime 이미지의 위험도 분류와 `Devices/`의 available/unavailable 기기 구분(simctl 필요), Linux 전체 지원, 커널 드라이버·파일시스템 minifilter 구현, 모든 파일 읽기/쓰기 이벤트 추적, 시스템 구성요소(Windows SDK/Visual Studio/.NET Runtime/Xcode 등) 강제 삭제, 사용자 문서·데이터베이스·Docker Volume의 자동 삭제, GUI 애플리케이션.

---

## 실행 방법

### 준비물

- Go 1.25 이상 (`go.mod`의 `go 1.25.0` 기준)
- Windows 10/11 권장 — Windows SDK/Visual Studio/MSBuild 등 전체 리소스 탐지 기능을 쓰려면 Windows가 필요하지만, 그 외 명령(Git/Node/Python 프로젝트 탐지, plan/clean/restore 등)은 다른 플랫폼에서도 빌드·실행된다.

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

### 전역 옵션

모든 하위 명령이 공유하는 persistent flag (`cmd/root.go`):

| 옵션 | 설명 |
|---|---|
| `--config <path>` | 설정 파일 경로 (기본값: `.libra.yaml`) |
| `--json` | 결과를 표준 JSON envelope로 출력 |
| `--verbose` | 추가 진단 정보 출력 (예: `scan`의 모든 warning) |
| `--no-color` | 텍스트 출력에서 ANSI 색상 비활성화 |

`--yes`(대화형 확인 프롬프트 생략)는 전역이 아니라 실제로 확인 프롬프트가 있는 `clean --execute`, `purge --execute` 두 명령에만 로컬 플래그로 존재한다. `clean`/`purge`는 별도의 `--dry-run` 플래그 없이 `--execute`를 주지 않으면 항상 미리보기(dry-run) 모드로 동작한다.

### 명령어 실행 순서와 사용법

아래는 실제 사용 흐름을 따라간 순서다. 1~2번은 최초 1회만 필요하고, 3번(`scan`) 이후 4번의 조회 명령들은 몇 번이든 자유롭게 반복해도 되며, 5번 이후는 실제로 공간을 정리하고 싶을 때만 필요하다.

**1. `libra init`** — 최초 설정. `.libra.yaml`이 없으면 위험한 시스템 경로가 자동 제외된 기본 설정(제외 경로, scan/cleanup 기본값)을 생성하고, 로컬 SQLite DB(`.libra.db`)를 준비한다. 이미 파일이 있으면 그대로 두고 DB만 확인한다.
```bash
libra init
libra init --config .libra.yaml
```
결과물: `.libra.yaml`, `.libra.db`. `init` 실행 후에는 `.libra.yaml`의 `project_roots`가 비어 있으므로 직접 스캔할 경로를 채워 넣어야 한다(또는 매번 `scan --root`로 지정).

`init`을 아직 실행하지 않은 디렉터리에서는 `init`/`help`/`completion`을 제외한 모든 명령이 전역 가드(`cmd/root.go`의 `requireInit`, `PersistentPreRunE`)에 의해 즉시 차단되고 `libra is not initialized here`와 함께 `libra init`을 먼저 실행하라는 안내가 출력된다. 이 검사는 `daemon start`/`config show`처럼 하위 명령(subcommand)에도 동일하게 적용된다.

**2. `libra config show|validate|set`** — 유효 설정을 확인·검증하거나 안전하게 수정한다. `set`은 `project_roots`/`exclude`(쉼표 구분), `scan.max_depth`/`scan.stale_days`/`scan.follow_reparse_points`, `cleanup.quarantine_days`를 지원하고 저장 전에 전체 설정을 검증한다.
```bash
libra config show
libra config validate
libra config set project_roots D:\Projects,D:\Work
libra config set scan.max_depth 30
```

**3. `libra scan [--root <path>]`** — 실제 파일시스템 스캔과 분석을 실행하는 유일한 명령(DB에 쓰는 것도 이 명령뿐). 동작 순서: 설정된 루트(또는 `--root`)를 병렬 워커 4개로 순회 → Git/Node/MSBuild/Python/Gradle/Maven/Cargo/Go/Xcode/SwiftPM 프로젝트 탐지 → Windows SDK/.NET SDK/Visual Studio/Docker/생태계 어댑터로 리소스 탐지 → 논리 크기 계산 → MSBuild/Conda/Xcode 의존성 분석기 실행 → 결과를 SQLite에 저장한다. scan/project/resource/dependency/evidence/issue를 SQLite에 저장. 경로 접근 오류가 나도 전체 스캔은 중단되지 않고 issue로 기록된다.
```bash
libra scan
libra scan --root D:\Projects
```
결과물: `Roots scanned / Projects found / Resources found / Files inspected` 요약과 발견된 warning 일부(`--verbose`로 전체), 마지막 줄에 다음 단계 안내(`Next: libra summary`). 데몬은 변경된 root에 `--root`를 적용해 증분 갱신한다.

`--json`이 아닐 때는 스캔 도중 stderr에 실시간 진행률 바(`cmd/scan_progress.go`)가 표시된다. 파일 탐색 단계는 `Scanning... [====------] 42%`처럼 채워지는 바와 `files: N, directories: N` 카운트를 보여주는데, 같은 root 조합으로 완료된 직전 scan이 있으면 그 파일 수를 목표치로 삼아 퍼센트를 계산하고(determinate), 그런 기준이 없으면 좌우로 움직이는 bar로 "동작 중"만 표시한다(indeterminate). 탐색이 끝나면 `Scan 100%`로 고정되고, 이후 프로젝트 분석·리소스 탐지·의존성 해석·위험도 계산·저장 등 각 단계가 시작될 때마다 이전 단계는 `<단계명> 100%`로 확정되어 아래에 쌓이고 다음 단계 라벨이 새로 뜬다. ANSI 커서 이동으로 같은 자리에 다시 그리는 방식이라 Windows 10 1511+ 등 ANSI를 지원하는 터미널이 필요하며, 스캔이 끝나면 이 블록 전체가 지워지고 최종 요약이 출력된다.

**4. 저장된 스캔 결과 조회** — 아래 5개는 모두 read-only이며 직전 `scan`이 채운 SQLite만 읽는다.

- `libra summary [--drive C:] [--type node-modules]` — 리소스 타입/드라이브별 용량 합계와 안전하게 확보 가능(`SAFE`)/검토 필요(`REVIEW`)/차단(`BLOCKED`) 총량, 마지막 scan 시각과 완료 상태(`Complete`/`Partial · N warning(s)`/`Incomplete`)를 보여준다. `scan`을 한 번도 실행하지 않았다면 이 상태가 `Not scanned yet`으로 명시되어, "스캔 결과 0건"과 "아직 스캔 안 함"을 구분할 수 있다. 언제: 전체 현황을 한눈에 보거나 `plan --target`을 정할 근거가 필요할 때.
```bash
libra summary
libra summary --drive C:
libra summary --type node-modules
```

- `libra projects [--all] [--sort size|modified] [--name <substr>] [--under <path>] [--type] [--drive] [--status]` — 발견된 프로젝트 목록(이름/경로/타입/드라이브/크기/최종 수정·관측 시각/활동 상태/의존 리소스 수). 기본은 상위 20개만 보여주며 `--all`로 전체를 본다. 언제: 어떤 프로젝트가 있는지, 오래 방치된(`STALE`) 프로젝트가 뭔지 확인할 때.
```bash
libra projects
libra projects --all
libra projects --sort size
libra projects --name frontend
libra projects --under D:\Work
libra projects --status stale
```

- `libra resources [--type windows-sdk] [--risk review]` — 발견된 SDK·도구·캐시·산출물 목록(이름/타입/버전/경로/크기/의존 프로젝트 수/재생성 가능 여부/위험도/신뢰도/risk reason). 언제: 특정 타입이나 위험도로 좁혀서 무엇이 있는지 볼 때.
```bash
libra resources
libra resources --type windows-sdk
libra resources --risk review
```

- `libra issues [--scan <scan-id>] [--code ACCESS_DENIED] [--severity warning]` — 최신(또는 지정한) 스캔이 남긴 구조화된 경고·오류 목록. 언제: `summary`의 "Warnings: N"이 정확히 어떤 경로에서 왜 발생했는지 확인할 때.
```bash
libra issues
libra issues --scan scan-20260721-120000
libra issues --code ACCESS_DENIED --severity warning
```

- `libra explain <target>` — 프로젝트 또는 리소스 하나를 골라 종류·경로·크기·근거·영향·복구법·위험도·신뢰도를 전부 보여준다. `<target>` 문법: `<타입>:<버전>`(예: `windows-sdk:10.0.22621.0`), `project:"경로 또는 이름"`, 따옴표로 감싼 절대 경로, 또는 ID/이름. 언제: "이 폴더/SDK가 정확히 뭔지, 왜 여기 있는지" 알고 싶을 때.
```bash
libra explain windows-sdk:10.0.22621.0
libra explain "D:\Projects\OldWeb\node_modules"
libra explain project:"D:\Projects\GameClient"
```

- `libra impact <resource-target>` — 리소스 하나를 제거했을 때 그것에 의존하는 모든 프로젝트별 `RUN`/`BUILD`/`DEBUG` 영향 수준(`NONE`/`LOW`/`HIGH`/`UNKNOWN`)과 복구 방법을 보여준다. 언제: "이거 지우면 어떤 프로젝트들이 정확히 어떻게 깨지는가"를 확인할 때.
```bash
libra impact windows-sdk:10.0.22621.0
libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"
```

**5. `libra plan [--target 10GB] [--risk safe|review|blocked] [--project <path>]`** — `SAFE` 후보만 신뢰도 → 크기 순으로 목표 용량(`--target`, 생략 시 무제한)에 도달할 때까지 자동 선택해 cleanup plan을 SQLite에 저장하고 Plan ID를 출력한다. `REVIEW`/`BLOCKED`는 절대 자동 선택되지 않고 왜 제외됐는지와 함께 표시만 된다.
```bash
libra plan
libra plan --target 10GB
libra plan --risk safe
libra plan --project D:\Projects\OldWeb
```
결과물: Plan ID + `SAFE`/`REVIEW`/`BLOCKED` 후보 목록. 언제: "안전하게 N GB를 확보하려면 뭘 지워야 하나"에 답할 때. 이후 `clean --plan <id>`에 이 Plan ID를 넘긴다.

**6. `libra clean --plan <id> [--execute [--yes]]`** — 기본(플래그 없이)은 dry-run: plan에 저장된 각 `SAFE` 항목을 현재 상태와 재대조해 "그대로 이동 가능(`would-move`)/크기·위험도가 바뀜(`changed`)/더 이상 존재하지 않음(`missing`)"을 보여줄 뿐 아무것도 건드리지 않는다. `--execute`를 주면 재검증을 통과한 항목만 같은 볼륨의 `.Libra-quarantine\tx-<id>\`로 이동(삭제 아님)하고 transaction을 기록한다. `--yes` 없이 실행하면 대화형 확인을 먼저 묻는다.
```bash
libra clean --plan plan-20260717-001
libra clean --plan plan-20260717-001 --execute --yes
```
언제: `plan`으로 나온 계획을 실제로 실행하기 직전 미리보기, 그리고 승인 후 실행.

**7. `libra transactions`** — 지금까지의 격리·복구·퍼지 transaction과 각 항목 상태(`MOVED`/`RESTORED`/`PURGED`/`FAILED` 등) 목록을 보여준다.
```bash
libra transactions
```
언제: 이전에 어떤 `clean`/`restore`/`purge`를 실행했는지, transaction ID가 무엇이었는지 다시 찾을 때.

**8. `libra restore --transaction <id>`** — 격리된 항목을 원래 경로로 되돌린다. 원래 위치에 이미 무언가 있으면 덮어쓰지 않고 건너뛴다.
```bash
libra restore --transaction tx-20260717-001
```
언제: `clean --execute`로 격리한 항목이 실제로 필요했다는 게 나중에 밝혀졌을 때.

**9. `libra purge --transaction <id> [--execute [--yes]]`** — 격리 후 보존 기간(`quarantine_days`, 기본 7일)이 지난 `QUARANTINED` transaction만 대상이며, 기본은 삭제 후보 목록만 보여주는 dry-run이고 `--execute`(+확인 프롬프트, `--yes`로 생략 가능)를 줘야 실제 영구 삭제가 일어난다.
```bash
libra purge --transaction tx-20260717-001
libra purge --transaction tx-20260717-001 --execute --yes
```
언제: 격리 기간이 지나 정말 더는 필요 없다고 확신했을 때. **되돌릴 수 없는 유일한 명령이므로 가장 신중하게 사용해야 한다.**

**10. `libra export [--format json|markdown] [--output <file>]`** — 최신 scan의 project/resource/issue/transaction 요약을 파일 또는 표준 출력으로 내보낸다.
```bash
libra export --format json
libra export --format markdown --output report.md
```
언제: 발표 자료, 버그 리포트, 팀원 간 환경 공유, 후속 분석 입력이 필요할 때. (`--format markdown`은 `--json`과 함께 쓸 수 없다.)

**11. `libra daemon start` / `libra daemon status` / `libra daemon stop`** — 설정된 `project_roots`의 파일 인벤토리를 2초 간격으로 비교해 변경 root만 증분 스캔한다. `status`는 실행 상태와 마지막 스캔·오류를 보여준다. 자동 정리는 하지 않는다.
```bash
libra daemon start
libra daemon status
libra daemon stop
```
언제: 코딩하는 동안 저장공간 현황이 거의 실시간으로 갱신되길 원할 때.

**12. `libra events [--kind CREATE] [--since 24h] [--limit 50]`** — 데몬이 기록한 파일 이벤트와 증분 스캔 이력을 조회한다. `--since`는 RFC3339 시각 또는 `24h` 같은 기간을 받는다.
```bash
libra events
libra events --since 24h
libra events --kind SIZE_CHANGE --limit 10
```
언제: 데몬이 실제로 언제 무엇을 감지해서 재스캔했는지 확인할 때.

### 개발 중 검증

PR을 올리기 전에 아래를 통과시켜야 한다(자세한 규칙은 `docs/libra_collaboration_rules.md` 참고).

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./...
```

---

## 기술 스택

| 분류 | 사용 기술 |
|---|---|
| 언어 | Go 1.25 (`go.mod`: `go 1.25.0`) |
| CLI 프레임워크 | [spf13/cobra](https://github.com/spf13/cobra) v1.10.2 (+ `spf13/pflag`, `inconshreveable/mousetrap`, indirect) |
| 데이터 저장 | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) v1.53.0 — 순수 Go SQLite 드라이버(cgo 불필요, Windows/macOS 크로스 빌드 단순화). 스키마는 `internal/store/sqlite/migrations/*.sql`을 `embed.FS`로 임베드해 파일명 순서대로 순차 적용하고 적용 이력을 `schema_migrations` 테이블에 기록 |
| 설정 | YAML, [go.yaml.in/yaml/v3](https://github.com/yaml/go-yaml) v3.0.4 — 알 수 없는 필드는 로드 시 거부(`KnownFields(true)`) |
| 크기/경로 유틸 | [dustin/go-humanize](https://github.com/dustin/go-humanize) v1.0.1(`--target 10GB`처럼 사람이 읽는 크기 파싱·표시), [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) v0.40.0(NFC 경로 정규화로 겉보기엔 같은 경로를 동일하게 비교) |
| ID 생성 | [google/uuid](https://github.com/google/uuid) v1.6.0 (indirect) |
| Windows 통합 | `vswhere.exe`(Visual Studio·MSBuild 설치 위치 탐지), `dotnet --list-sdks`/`--list-runtimes`, `docker system df --format '{{json .}}'`, `sdkmanager`/`conda env list` 등 외부 CLI 결과 파싱 |
| 테스트 | Go 표준 `testing`, golden output 테스트(`summary`/`plan`/`impact`), fixture 기반 통합 테스트, 성능 테스트(`dependency_repository_performance_test.go`) |
| CI/CD | GitHub Actions — Windows·macOS 두 러너에서 빌드·테스트 (Windows 전용 어댑터는 다른 플랫폼에서 `RequireWindows`로 명시적 미지원 처리되어 CI가 깨지지 않음) |
| 배포 형태 | 단일 실행 파일(`go build -o libra .`), 외부 런타임 의존성 없음 |

---

## 데이터 구조

`libra`는 SQLite 단일 파일(`.libra.db`)에 모든 분석 결과를 저장한다. 마이그레이션은 `internal/store/sqlite/migrations/001_initial.sql`부터 `012_resource_confidence_freshness.sql`까지 파일명 순서대로 적용되며, `schema_migrations` 테이블이 적용 이력을 기록해 재실행 시 중복 적용을 막는다.

### 핵심 테이블

| 테이블 | 역할 |
|---|---|
| `scans` | 스캔 1회의 시작·종료 시각, 대상 루트, 파일 수, 오류 수, 상태 |
| `projects` | 발견된 프로젝트(이름/타입/루트·manifest 경로(원본·정규화)/드라이브/크기/최종 수정·관측 시각/활동 상태), `workspace_projects`를 통해 `workspaces`와 다대다 연결 |
| `workspaces` / `workspace_projects` | `.sln` 등 여러 프로젝트를 묶는 workspace와 그 소속 관계(하나의 프로젝트가 여러 workspace에 속할 수 있음) |
| `resources` | 발견된 SDK·도구·캐시·산출물(타입/버전/경로/크기/재생성 가능 여부/시스템 관리 여부/위험도), 6축 신뢰도 컬럼(`confidence_classification`/`_ownership`/`_dependency`/`_cleanup_safety`/`_scan_coverage`/`_freshness`)과 `risk_reasons`(JSON 배열) 포함 |
| `dependencies` | 프로젝트 ↔ 리소스 의존 관계 edge(`source`/`target` 타입·ID, relation, confidence) |
| `evidence` | 각 `dependency`를 뒷받침하는 근거(종류, 원본 경로, 속성명, 원본/해석된 값, 수집 시각) |
| `scan_issues` | 스캔 중 발생한 경고·오류(코드/phase/adapter/경로/operation/심각도/메시지) |
| `cleanup_plans` / `cleanup_items` | `plan`이 저장하는 계획 스냅샷(목표·선택 바이트, 상태) 및 그 항목별 스냅샷(정규화 경로, 예상 타입·크기·수정시각, 계획 시점 신뢰도·위험도, 소유 프로젝트, 재생성 명령) |
| `transactions` / `transaction_items` | `clean --execute`/`restore`/`purge`가 만드는 transaction(manifest 버전 포함)과 항목별 원본·격리 경로, manifest 경로, 상태(`MOVED`/`RESTORED`/`PURGED`/`FAILED` 등) |

### Go 도메인 모델 (`internal/domain`)

- **`Workspace` / `BuildProject`** — Workspace는 `.sln` 등 여러 BuildProject를 묶는 그룹(그 자체는 빌드 의존성이 없음), BuildProject는 실제로 분석 가능한 프로젝트 단위(`ProjectType`: `msbuild-cpp`/`msbuild-dotnet`/`node`/`git`/`python`, `ProjectStatus`: `ACTIVE`/`STALE`/`ARCHIVED`/`UNKNOWN`).
- **`Resource`** — `ResourceType`(`windows-sdk`/`netfx-sdk`/`visual-studio`/`msbuild`/`dotnet-sdk`/`android-sdk`/`node-modules`/`build-output`/`global-cache`/`docker-cache`/`docker-volume`/`python-venv`/`conda-env`), 논리 크기·재생성 가능 여부·`RiskLevel`(`SAFE`/`REVIEW`/`BLOCKED`)·`ConfidenceProfile`·`RiskReason` 목록·재생성 명령(`RegenerationCommand`)을 갖는다.
- **`Dependency` / `Evidence`** — 프로젝트→리소스 관계와 그 근거(evidence 종류·출처 경로·속성명·원본/해석값).
- **`ConfidenceProfile`** — `Classification`/`Ownership`/`Dependency`/`CleanupSafety`/`ScanCoverage`/`Freshness` 6축(각 0~100), `Overall()`은 최솟값을 반환.
- **`RiskReason`** — `BLOCKER`/`WARNING`/`SAFEGUARD`/`UNKNOWN` 심각도와 코드·메시지로 위험도 판단 근거를 구조화.
- **`ImpactAssessment`** — `RUN`/`BUILD`/`DEBUG`/`CI` 범위(`ImpactScope`)별 `NONE`/`LOW`/`HIGH`/`UNKNOWN` 판단(`ImpactLevel`).
- **`CleanupPlan` / `CleanupPlanItem`, `CleanupTransaction` / `CleanupTransactionItem`** — `plan`/`clean`/`restore`/`purge`가 공유하는 계획·트랜잭션 스냅샷과 상태 전이(`PLANNED`→`RUNNING`→`QUARANTINED`/`PARTIALLY_QUARANTINED`→`RESTORED`/`PURGED`/`PARTIALLY_*`/`FAILED`).
- **`UnverifiedScope`** — 평가되지 않은 분석 범위(예: `Condition=` 게이트가 있는 MSBuild 속성)를 "평가 안 됨"과 "평가했더니 없음"을 구분해 기록.
