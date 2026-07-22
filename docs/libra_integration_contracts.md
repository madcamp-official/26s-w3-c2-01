# Libra 통합 계약

> 기준일: 2026-07-20
>
> 상태 표기: `IMPLEMENTED`는 코드와 테스트가 존재하고, `CONFIRMED`는 합의됐지만 일부 구현이 남았으며, `PLANNED`는 후속 범위다.
> 이 문서는 과거 제안과 현재 계약을 섞지 않는 유일한 통합 기준이다. 변경 시 코드·migration·테스트·사용자 문서를 같은 PR에서 갱신한다.

## 1. 제품 경계와 안전 원칙

Libra는 프로젝트, SDK, 도구, 캐시와 빌드 산출물을 스캔해 관계와 근거를 SQLite에 저장하고 `summary → explain → impact → plan → clean → restore` 흐름을 제공한다.

- 읽기 명령은 파일시스템을 변경하지 않는다.
- `clean` 기본값은 dry-run이다.
- 자동 정리 대상은 검증이 끝난 프로젝트 산출물뿐이다.
- 직접 삭제하지 않고 같은 volume의 quarantine으로 이동한다.
- 하나라도 미검증이면 `REVIEW`, 시스템 관리 또는 denylist 대상이면 `BLOCKED`다.
- 복구 근거의 원본은 disk manifest이며 DB는 조회 index다.
- cleanup·restore와 시스템 경로 정책은 Windows A를 포함한 2명 이상이 리뷰한다.

## 2. 계층과 소유권

```text
cmd → internal/app → internal/domain → adapter/repository
```

| 영역 | 담당 |
|---|---|
| scanner, SQLite, config, safety, cleanup/restore | Windows A |
| Windows SDK, MSBuild, Visual Studio, .NET | Windows B |
| CLI, output, Node, fixture, docs | Mac C |
| domain, DB schema, CLI/JSON 계약 | 공동 |

`cmd`는 orchestration과 입출력만 담당한다. 판정은 application/safety에, SQL은 SQLite repository에, 표현은 output에 둔다.

## 3. 경로와 ID (`IMPLEMENTED`)

모든 identity 비교는 `internal/pathutil.Normalize`를 사용한다. 절대·clean 경로로 만들되 symlink/junction을 resolve하지 않는다. Windows identity는 대소문자를 무시한다. UI에는 `DisplayPath`, 비교와 ID에는 `NormalizedPath`를 쓴다.

| 모델 | stable ID 입력 |
|---|---|
| Project | `ProjectType + NormalizedManifestPath` |
| Workspace | `WorkspaceType + NormalizedManifestPath` |
| Resource | `ResourceType + Version + NormalizedPath` |
| Dependency | typed source + relation + typed target |
| Evidence | dependency + kind + source/property/value |

ID는 NUL 구분 직렬화의 SHA-256 hex다. 관찰 시각은 stable ID에 넣지 않는다.

## 4. 공용 domain (`IMPLEMENTED`)

### Project와 Workspace

- `BuildProject`: 직접 빌드·분석 가능한 단위
- `Workspace`: `.sln` 같은 그룹 파일
- 한 Project는 여러 Workspace에 속할 수 있다.
- 지원 project type: `msbuild-cpp`, `msbuild-dotnet`, `node`, `git`, `python`, `gradle`, `maven`, `cargo`, `go`, `android`, `xcode`(.xcodeproj), `swiftpm`(Package.swift)
- 지원 workspace type: `vs-solution`(.sln), `node`, `xcode-workspace`(.xcworkspace)
- 상태: `ACTIVE`, `STALE`, `ARCHIVED`, `UNKNOWN`

### Resource

지원 type은 `windows-sdk`, `netfx-sdk`, `visual-studio`, `msbuild`, `dotnet-sdk`, `android-sdk`, `node-modules`, `build-output`, `global-cache`, `docker-cache`, `docker-volume`, `python-venv`, `conda-env`, `xcode-install`, `cocoapods-pods`다.

크기는 `LogicalSize`, `SizeKnown`, `ReclaimableSize`로 구분한다. `ReclaimableSize`는 중앙 RiskPolicy가 SAFE로 판정한 경우에만 실제 정리 가능 용량이 된다.

### Graph

```text
PROJECT --REQUIRES--> RESOURCE
PROJECT --OWNS------> RESOURCE
```

`REQUIRES`는 build/run 영향 분석용, `OWNS`는 cleanup 소유권용이다. 프로젝트와 함께 발견한 산출물은 스캔 중 stable project/resource ID를 만든 뒤 `OWNS` edge와 `OBSERVED` Evidence를 저장한다.

### Evidence와 Confidence

| EvidenceKind | 기본 Confidence |
|---|---:|
| RESOLVED | 90 |
| OBSERVED | 85 |
| DECLARED | 75 |
| PINNED | 60 |
| INFERRED | 40 |
| UNKNOWN | 10 |

`PINNED`는 §19.4(Python)에서 추가한 등급으로, DECLARED(실제 lockfile)와
INFERRED(근거 없음) 사이 — 버전이 전부 고정된 `requirements.txt`처럼
"lockfile은 아니지만 재현성 근거는 있는" 경우를 표현한다.

Confidence는 분석 coverage이지 실제 확률을 뜻하지 않는다. 단일 점수는 호환용 요약이며,
새 resource 관측은 `Classification`, `Ownership`, `Dependency`, `CleanupSafety`,
`ScanCoverage`, `Freshness` 여섯 축의 `ConfidenceProfile`을 저장하고 최솟값을 요약값으로 사용한다
(`IMPLEMENTED`). 기존 DB row는 migration 시 기존 confidence를 각 축에 복사한다.

`Freshness`는 확률이 아니라 마지막 관측의 나이 등급이다. 7일 이하는 100, 30일 이하는
80, 90일 이하는 50, 그 이후는 20이며 관측 시각이 없으면 0이다. 30일을 초과한 `SAFE`는
현재 조회와 plan 생성 시 `REVIEW`로 낮추고 `EVIDENCE_STALE` unknown을 추가한다. 새 scan은
관측 시각과 freshness를 다시 100으로 갱신한다 (`IMPLEMENTED`).

`RiskAssessment`는 blocker/warning/safeguard/unknown으로 분류된 `RiskReason`을 반환하며
critical unknown이 있으면 cleanup evidence가 완전해도 `REVIEW`다 (`IMPLEMENTED`). 수집된
모든 `UnverifiedScope`를 resource별 unknown/coverage 감점으로 변환하는 pipeline 연결은
`CONFIRMED`이며 후속 구현 범위다. 복수 Evidence 결합 공식과 운영 결과 기반 점수 보정은
P2로 남기며 이번 변경 범위에 포함하지 않는다.

`resources --json`, resource `explain`, `plan`은 저장된 `RiskReason`을 그대로 노출한다.
JSON은 code/severity/message 구조를 보존하고 텍스트 출력은 message를 `Reason:`으로 요약한다
(`IMPLEMENTED`, issue #40). 별도 문자열 reason column은 두지 않는다.

## 5. Scanner와 분석 pipeline (`IMPLEMENTED`)

```text
DISCOVER_FILES
→ DISCOVER_PROJECTS
→ DISCOVER_SYSTEM_RESOURCES
→ ANALYZE_PROJECT_SETTINGS
→ RESOLVE_DEPENDENCIES
→ CLASSIFY_ARTIFACTS
→ CALCULATE_RISK
→ PERSIST_RESULTS
→ COMPLETED
```

- scanner는 symlink와 Windows reparse point를 따라가지 않는다.
- 접근 실패는 structured Issue로 수집하고 가능한 범위에서 계속한다.
- adapter 반환은 `DetectionResult[T]`로 통일한다.
- MSBuild declared property는 실제 manifest/`Directory.Build.props` SourcePath를 보존한다.
- Windows SDK/.NET SDK dependency analyzer는 실제 scan에 연결돼 Evidence와 graph를 저장한다.
- Xcode/SwiftPM dependency analyzer(§19.9)도 동일하게 연결돼 있다.
- scan record가 완료되기 전에 project/resource/dependency 결과를 저장한다.

Incremental snapshot 전환, 미발견 project의 자동 STALE 처리와 scan 동시 실행 lock은 `PLANNED`다.

## 6. Repository와 transaction (`IMPLEMENTED`)

- Project: scan 단위 `UpsertObserved`, ID/manifest 조회, 목록
- Resource: `Upsert`, ID/type/전체 조회
- Dependency: edge와 Evidence를 하나의 DB transaction으로 upsert
- CleanupPlan: plan과 모든 item snapshot을 하나의 transaction으로 생성
- CleanupTransaction: transaction/item 상태를 함께 생성·갱신·조회
- migration은 번호 순서로 적용하고 `schema_migrations`에 기록한다.

현재 schema migration은 `001`부터 `010`까지다. 기존 row를 파괴하지 않고 column/table을 추가한다.

`009_scan_issues.sql`은 스캔 중 수집된 structured Issue를 별도 실행에서도 조회할 수 있도록
`scan_issues(scan_id, code, phase, adapter, path, operation, severity, message)`를 추가한다.
`scan_id`는 `scans(id)`를 참조하고 scan 삭제 시 함께 삭제된다. 한 스캔의 issue 목록은
완료 또는 실패 시 transaction 안에서 전체 교체하며, 프로세스 내부 error chain인 `Cause`는
영속화하지 않는다 (`IMPLEMENTED`, issue #47).

`010_resource_risk_assessment.sql`은 다섯 confidence 축과 JSON `risk_reasons`를 resources에
추가한다. 기존 row는 기존 confidence를 모든 축에 복사해 호환성을 유지한다 (`IMPLEMENTED`).

## 7. Risk와 CleanupEligible (`IMPLEMENTED`)

```text
CleanupEligible =
    ProjectOwned
    AND KnownOutputPath
    AND Regenerable
    AND NOT SystemManagedOrProtected
    AND ReparsePointFree
    AND GitTrackedOriginalsAbsent
    AND ExecutionRevalidationPassed
```

RiskPolicy 입력의 기본 evidence는 다음 네 가지다.

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

> 부분 해결(2026-07-20): `Directory.Build.props` 상속 property는 이제 실제로
> 읽힌다. `XMLBuildProjectParser`가 프로젝트 디렉터리부터 상위로 올라가며
> 가장 가까운 `Directory.Build.props` 하나를 찾아 파싱하고, 프로젝트 파일
> 자신이 같은 이름의 property를 선언하면 상속값을 버리는 방식으로 override를
> 구현했다(실제 MSBuild의 `<Import>` 위치 기반 평가 순서 전체를 재현하지는
> 않음 — "프로젝트 자신의 파일이 항상 우선"이라는 단순 규칙). `DeclaredProperty.SourcePath`가
> 실제 원본 파일(프로젝트 파일 또는 props 파일)을 가리키므로 §15.1의
> "Directory.Build.props는 manifest가 아닌 Evidence source"라는 기존 결정과
> 일치한다. Configuration/Platform 변수 치환, 조건부 속성 평가, 여러 단계로
> 체이닝된 `Directory.Build.props`는 여전히 `DECISION_REQUIRED`로 남아있다.

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

### 19.2 Node workspace (`CONFIRMED`, Option C — 2026-07-21)

```text
workspace 경계 밖 package.json → 독립 BuildProject 후보
workspace root              → Workspace + BuildProject
명시적으로 선언된 member     → BuildProject + WorkspaceProject
workspace 내부 미선언 package → 프로젝트 후보에서 제외
root node_modules → workspace 소유 Resource
하위 node_modules → 해당 package 소유 Resource
```

Option C의 프로젝트 선택 규칙은 다음으로 고정한다.

1. Node workspace 경계 밖에서 발견된 `package.json`은 독립 프로젝트로 유지한다. 넓은 상위 디렉터리를 scan root로 사용하는 기존 동작을 깨지 않기 위해 scan root의 직계 경로만으로 제한하지 않는다.
2. `package.json#workspaces` 또는 `pnpm-workspace.yaml`을 선언한 디렉터리는 workspace root이자 직접 빌드 가능한 Node `BuildProject`다.
3. workspace root 아래에서는 `ResolveMembers`가 해당 workspace 선언으로 실제 해석한 member만 `BuildProject`로 인정한다. 단순히 중첩된 `package.json`이 있다는 이유만으로 프로젝트가 되지 않는다.
4. 미선언 하위 프로젝트의 project property, 소유 Resource, OWNS edge도 함께 제외한다. 필터링은 DB 영속화와 resource 측정 전에 수행한다.
5. workspace는 `WorkspaceTypeNode`로 저장하고, npm/Yarn은 root `package.json`, pnpm은 `pnpm-workspace.yaml`을 안정 ID의 manifest로 사용한다. member 관계는 기존 `WorkspaceProject` 다대다 저장소를 재사용한다.
6. 중첩 workspace는 기존 MVP 범위대로 재귀 확장하지 않는다. 바깥 workspace가 member로 명시한 root까지만 인정하며, 중첩 workspace 재귀 지원은 추후 별도 합의한다.

이 규칙은 Option D의 `Origin` 필드를 도입하지 않는다. 화면 표시용 origin은 별도 기능이며, 프로젝트/DB/그래프 오염을 막는 Option C의 정확성 계약과 분리한다.

> 갱신(2026-07-21, issue #36): `node_modules` 하위(또는 내부)의 `package.json`은
> 설치된 third-party 의존성이지 개발자 프로젝트가 아니므로 BuildProject으로
> 탐지하지 않는다. Node adapter의 `CanDetect`가 경로에 `node_modules` 세그먼트가
> 있으면 프로젝트 후보에서 제외한다(`isVendoredPath`). 위 매핑의 "각 package.json
> → BuildProject 후보"는 vendored 경로를 제외한 뒤에 적용된다. 소유 프로젝트의
> `node_modules`는 여전히 `DetectArtifacts`로 Resource로 잡히고
> `scanner.MeasureResource`로 크기를 재는데, 둘 다 스캔 walk가 node_modules 안으로
> 내려가는 것에 의존하지 않는다. issue #36의 Windows A 범위도 구현해 기본
> exclude가 `node_modules`, `.next`, `dist`, `build`, `bin`, `obj`, `.git`,
> `.libra-quarantine`을 어느 깊이에서든 path segment 단위로 가지치기한다.
> 사용자가 config에 `exclude`를 명시하면 해당 목록으로 기본값을 대체한다.

Node adapter 구현 전에 결정하기로 했던 6개 항목을 `internal/adapter/node`
(Mac C 소유 영역)에서 MVP 범위로 확정하고 구현했다. 처음에는 workspace
지원 자체를 범위 밖으로 미뤘었는데("관계/공용 자원 연결을 지원해야 하지
않냐"는 지적으로) 이번에 다시 열어서 아래처럼 재결정했다.

| 항목 | MVP 결론 |
|---|---|
| npm/pnpm/Yarn workspace 지원 범위 | **지원(재결정).** `package.json`의 `workspaces` 필드(npm/Yarn, 배열 또는 `{packages:[...]}` 객체 형태 모두 인정)와 `pnpm-workspace.yaml`(pnpm)을 읽어 workspace root를 탐지한다(`DetectWorkspace`). member는 `filepath.Glob` 기반 단일 세그먼트 glob만 지원한다 — `packages/*`는 되지만 재귀 `**`는 세그먼트 하나로만 매칭되고, `!` 부정 패턴은 적용되지 않고 그냥 건너뛴다(제외 안 됨, 안전한 쪽으로 미지원). 중첩 workspace(member가 또 다른 workspace root인 경우)는 한 단계만 풀고 재귀하지 않는다. |
| 여러 lockfile의 우선순위 | 불필요. `package-lock.json`/`npm-shrinkwrap.json`/`pnpm-lock.yaml`/`yarn.lock` 중 하나라도 있으면 재생성 근거로 충분하다고 본다(존재 여부만 확인, 어떤 패키지 매니저인지는 판단하지 않음). workspace member는 자기 디렉터리뿐 아니라 workspace root의 lockfile도 근거로 인정한다(`DetectMemberArtifacts`) — 실제로 npm/Yarn/pnpm workspace는 보통 lockfile을 root에 하나만 둔다. |
| lockfile 없는 node_modules의 재생성 가능성 | `Regenerable=false`. `Confidence`도 낮춰서(§20.2 확정 전 임시값) INFERRED 수준으로 취급한다. |
| malformed package.json 저장 방식 | `Detector.Detect`가 error를 반환한다. `NodeProjectDetector`가 `IssueMalformedManifest`와 `UnverifiedScope`로 수집하며, 다른 후보나 전체 scan을 막지 않는 recoverable 실패로 처리한다(§5). workspace 선언 또는 member 해석 실패도 같은 방식으로 해당 경계를 미검증 상태로 남긴다. |
| nested node_modules 탐색 | **부분 지원.** Option B의 discovery walk는 어느 깊이에서든 `node_modules`를 가지치기한다. 각 retained project의 바로 아래 산출물은 별도 `DetectArtifacts` 호출로 발견하고 `MeasureResource`가 독립적으로 전체 크기를 재므로 root `node_modules`의 내부 크기는 누락되지 않는다. workspace member에 별도 `node_modules`가 있으면 그 member 소유 Resource가 되며, root 공유 관계는 아직 graph에 연결하지 않는다. |
| `.pnpm` store 크기 소유권 | 범위 밖. 전역 pnpm store 분석은 원래 일정에서도 P1(`pnpm 전역 저장소`)이라 이번 결정에 포함하지 않는다. |

**남은 구현 사항**: member가 workspace root의 공유 `node_modules`를 쓴다는
`MemberArtifacts.SharesRootNodeModules` 정보는 아직 `PROJECT -> RESOURCE
REQUIRES` graph로 저장하지 않는다. 안정적인 Project/Resource ID와 dependency
저장 계약은 이미 확정·구현됐으므로, 후속 Node dependency analyzer가 이 값을
`RelationRequires` edge와 Evidence로 변환하면 된다. 이 후속 작업은 Option C의
프로젝트 선택 정확성과 분리하며, 구현 전 공유 자원의 재생성 명령·Evidence kind를
합의한다.

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
처리하지 않는다.

> 갱신(2026-07-20, 1차): 1·3·4번은 실제로 검증된다.
> `internal/app/project_detector_adapters.go`의 `projectArtifactCleanupEvidence`가
> Node/MSBuild 양쪽에서 공유하는 헬퍼로, `ProjectOwned`(1번)는 project root
> 밑에서 발견됐다는 사실로 채우고, `ReparsePointFree`(3번)는
> `internal/scanner.IsLinkLike`(기존 scan 내부 로직을 export)로, `GitTrackedOriginalsAbsent`(4번)는
> `internal/adapter/git.FindRepoRoot` + `TrackedFilesChecker`(`git ls-files`를
> 실제로 실행)로 검증한다. 확인이 실패하면(경로 stat 실패, git 미설치 등)
> 해당 evidence는 false로 남고 Issue로 기록된다 — 안전 쪽으로만 추측한다.
>
> 수정(2026-07-20, Mac에서 재현 후 확인): `msbuild.DetectArtifacts`/
> `node.detectArtifacts`가 `os.ReadDir`의 `DirEntry.IsDir()`(Lstat 기준이라
> symlink·reparse point는 무조건 false)로 후보를 걸러서, symlink/junction인
> `bin`/`node_modules`가 candidate로도 안 만들어지고
> `projectArtifactCleanupEvidence`의 reparse point 체크까지 아예 못
> 갔었다 — 3번 evidence가 사실상 도달 불가능한 죽은 코드였던 것. 실제 NTFS
> junction으로 검증해보니 `DirEntry.Type()&os.ModeSymlink`도 false라(Go가
> junction을 `ModeIrregular`로 분류) `ModeSymlink` 비트 체크로는 못 고치고,
> `os.Stat`(링크를 따라가는)으로 실제 디렉터리인지 다시 확인하는 방식으로
> 고쳤다. `resolveDeclaredOutputDirs`의 `OutDir`/`OutputPath` 값도 항상
> `\` 구분자로 선언돼 있어 macOS/Linux에서 `filepath.Clean`이 리터럴
> 문자로 취급해버리던 것을 `/`로 정규화 후 처리하도록 같이 고쳤다.

> 갱신(2026-07-20, 2차): 2번(output path)과 5번(재생성 Evidence)도 두
> adapter에서 각자의 현실적인 방식으로 강화했다.
>
> **MSBuild**: `WindowsTargetPlatformVersion`/`TargetFramework`와 같은
> declared-property 파이프라인을 `OutputPath`/`BaseOutputPath`/
> `IntermediateOutputPath`/`BaseIntermediateOutputPath`(.csproj)와
> `OutDir`/`IntDir`(.vcxproj)까지 확장했다(`internal/adapter/msbuild/artifacts.go`의
> `DetectArtifacts(root, declared)`). 프로젝트가 이 property를 무조건부로,
> MSBuild 매크로(`$(...)`) 없이 선언하고 그 경로가 실제로 존재하면
> `DECLARED` 수준 Confidence를 주고, `KnownOutputPath`는 이 Confidence가
> DECLARED 이상일 때만 `true`가 된다 — `bin`/`obj` 이름이 우연히 맞아도
> 설정으로 확인되지 않으면 `INFERRED`에 머문다. VC++(.vcxproj)는 애초에
> `bin`/`obj`가 기본값이 아니므로(보통 `$(Configuration)\` 계열) 이 구분이
> 특히 중요하다 — 매크로가 섞인 값은 어떤 Configuration으로 평가할지 알 수
> 없어 해석하지 않는다(Condition 미평가와 같은 정책).
>
> **Node**: 번들러(webpack/next/vite 등)마다 설정 파일 형식이 달라 output
> path 자체를 설정에서 확인하는 건 범위 밖으로 남겨뒀다(Day7 P1 "MSBuild
> preprocess 분석"과 같은 급의 작업). 대신 `Regenerable`을
> `package.json`의 `scripts.build` 존재 여부에 걸었다(`internal/adapter/node.hasBuildScript`) —
> `dist`/`.next`/`build`/`out`이 이름과 일치해도 build 스크립트가 없으면
> 재생성 증거가 전혀 없는 것이므로 더 이상 무조건 `Regenerable=true`를
> 주지 않는다. `node_modules`는 그대로 lockfile 존재로 판단(변경 없음).
>
> `KnownOutputPath`는 `node_modules`에 한해 예외적으로 항상 `true`다 —
> npm/Yarn/pnpm은 그 위치를 프로젝트가 바꿀 수 없게 고정해두므로, lockfile
> 유무로 달라지는 건 재생성 신뢰도이지 위치 확인 여부가 아니다.

### 19.4 Python (`CONFIRMED`, 2026-07-21)

세부 논의 근거는 `docs/libra_python_conda_scope_decisions.md`에 남겨두고, 이
절은 그 결과만 계약으로 확정한다. 담당은 `feature/python_scope` 브랜치
작업자(신규 소유 영역, `libra_collaboration_rules.md` §2 갱신 예정).

```text
project marker 우선순위   pyproject.toml > Pipfile > setup.py > requirements.txt
                        (requirements.txt 단독은 .py 파일이 있어야 인정)
재생성 evidence 등급     DECLARED(poetry.lock/Pipfile.lock/uv.lock)
                        > PINNED(버전 전부 고정된 requirements.txt, 신설)
                        > INFERRED(부분/미고정 requirements.txt, 또는 lock 없는 선언만)
                        > UNKNOWN(마커 없음)
venv 확인               .venv/venv/env 이름 + pyvenv.cfg 내용 확인 둘 다 필요
venv cleanup 허용        PINNED 이상 등급일 때만 Regenerable=true
캐시(__pycache__ 등)     항상 Regenerable=true, 이름 일치만으로 충분
```

`internal/adapter/python`이 project marker/venv/캐시를, project 소유
conda 로컬 prefix 환경과 `environment.yml` 파싱은 `internal/adapter/conda`가
맡고 `internal/app/project_detector_adapters.go`의 `PythonProjectDetector`가
둘을 NodeProjectDetector가 node.go+workspace.go를 잇는 것과 같은 방식으로
연결한다.

### 19.5 Conda (`CONFIRMED`, 2026-07-21)

```text
전역 named 환경     conda env list --json 으로 탐지, PROJECT --REQUIRES--> RESOURCE
                   (project의 environment.yml "name" 필드와 매칭될 때만 edge 생성)
로컬 prefix 환경    conda-meta/history 마커로 확인, PROJECT --OWNS--> RESOURCE
                   (§19.4 예외 — 위치 자체가 소유 증거)
cleanup 대상 여부   둘 다 항상 제외. RiskPolicy가 아니라 CleanupEvidence를
                   애초에 채우지 않는 방식으로 강제(§7 SAFE 경로 도달 불가)
이름 매칭 신뢰도    특정 프로젝트 이름 → DECLARED
                   base/env/py39 같은 일반적인 이름 → INFERRED + UnverifiedScope
conda 미설치        빈 결과, 에러 아님 (internal/adapter/dotnet.CLISDKLister와 동일
                   계약, 단 플랫폼 무관 — adapter.RequireWindows 가드 없음)
범위 제외 (PLANNED) pip 전역 캐시, conda 전역 pkgs 캐시
```

conda 환경은 project가 소유(OWNS)하더라도 cleanup 후보가 되지 않는다 —
`internal/app/project_detector_adapters.go`의 `PythonProjectDetector`가
로컬 prefix 환경에 대해 `projectArtifactCleanupEvidence`를 호출하지 않으므로
`CleanupEvidence`가 zero-value로 남고, `DefaultRiskPolicy.Classify`는
`Cleanup.complete()`가 거짓인 한 SAFE를 낼 수 없다. `internal/safety`의
cleanup allowlist에도 conda 관련 이름은 추가하지 않아 실행 직전 재검증
단계에서도 이중으로 막힌다.

### 19.6 Docker (`IMPLEMENTED`, read-only)

`internal/adapter/docker`는 설치된 Docker CLI로 `docker system df --format '{{json .}}'`를
실행해 Images, Containers, Build Cache, Local Volumes의 aggregate 사용량과 Docker가 보고한
reclaimable 크기를 읽는다. Docker CLI가 없으면 빈 결과이며 daemon 연결 실패나 JSON/크기
파싱 실패는 recoverable `ADAPTER_FAILED` scan issue다.

- Images, Containers, Build Cache: `docker-cache`, 항상 `REVIEW`
- Local Volumes: `docker-volume`, 항상 `BLOCKED` (`DOCKER_VOLUME_USER_DATA`)
- 모든 항목은 Docker CLI가 보고한 크기를 사용하고 host filesystem을 재순회하지 않는다.
- locator path는 현재 Docker CLI executable이며 identity의 일부로만 사용한다.
- `clean`, `purge`, daemon은 Docker prune/remove 명령을 실행하지 않는다.
- 정리는 Docker 공식 명령을 사용자가 별도로 검토·실행해야 한다.

### 19.7 Ecosystem SDK/cache adapters (`IMPLEMENTED`, analysis-only)

- Android SDK: `ANDROID_HOME`, deprecated fallback `ANDROID_SDK_ROOT`, 플랫폼 기본 경로 순으로 탐지하고 `android-sdk`/`BLOCKED`로 저장한다. SDK 변경은 Android Studio 또는 `sdkmanager`에 맡긴다.
- Gradle: `GRADLE_USER_HOME` 또는 `~/.gradle` 아래 `caches`만 `global-cache`로 탐지한다. `gradle.properties`, `init.d`, toolchain JDK는 포함하지 않는다.
- Cargo: `CARGO_HOME` 또는 `~/.cargo` 아래 `registry`, `git`만 탐지한다. credentials와 설치 binary, 프로젝트 `target`은 포함하지 않는다.
- Maven: `~/.m2/settings.xml`의 `localRepository` 또는 기본 `~/.m2/repository`를 탐지한다. settings/credentials 자체는 포함하지 않는다.
- npm: 설치된 CLI의 `npm config get cache` 결과를 사용한다.
- pnpm: 설치된 CLI의 `pnpm store path` 결과를 사용한다.

전역 cache는 모두 `REVIEW`이며 자동 plan/clean 대상이 아니다. CLI 부재는 해당 항목의 빈 결과이고, 실행·설정 파싱 실패는 recoverable scan issue다.

구현은 의미 경계를 유지하도록 `internal/adapter/android`, `gradle`, `cargo`, `maven`,
`npm`, `pnpm` package로 분리한다. 공통 filesystem helper만 `cachepath`에 둔다.
Libra는 cleanup 명령을 실행하지 않고 구조화된 `OFFICIAL_CLEANUP_GUIDANCE` reason으로
다음 안내만 제공한다: Android `sdkmanager --uninstall`, npm `npm cache clean --force`,
pnpm `pnpm store prune`, Maven `mvn dependency:purge-local-repository`. Gradle은 자체 자동
cleanup/retention 설정을 안내하고, Cargo는 전역 cache purge 명령이 없으므로 프로젝트
산출물에 한해 `cargo clean`을 안내한다.

### 19.8 macOS 개발 캐시 (`IMPLEMENTED`, read-only, 2026-07-21)

> 스코프 결정: README §2.2가 명시했던 "macOS와 Linux 지원 없음"은 프로젝트 타입 탐지·
> 의존성 그래프·clean/execute 전체 지원이 없다는 뜻으로 좁혀 유지한다. 이번 결정은 그
> 범위를 넘지 않는 **읽기 전용 macOS 개발 캐시 탐지**만 추가한다 — 기존 npm/pnpm/Gradle/
> Cargo/Maven 전역 캐시(§19.7)와 동일한 성격의 리소스이며, 새 `ResourceType`이나 DB
> migration 없이 기존 `global-cache`를 재사용한다.

- Xcode DerivedData: `~/Library/Developer/Xcode/DerivedData`. Xcode가 매 빌드마다 재생성하는 순수 캐시(인덱스·중간 산출물·미리보기)이며 프로젝트 소스가 아니다.
- CocoaPods: `~/Library/Caches/CocoaPods`.
- SwiftPM: `~/Library/Caches/org.swift.swiftpm` (macOS만 — Linux `~/.cache/org.swift.swiftpm` 경로는 실사용 검증 전이라 이번 범위에 포함하지 않는다).
- Homebrew: 설치된 `brew` CLI의 `brew --cache` 결과를 사용한다(npm/pnpm과 동일한 CLI-resolved 방식). Homebrew Cellar(설치된 formula/cask 본체)는 캐시가 아닌 시스템 구성요소이므로 탐지 대상에서 제외한다.
- iOS Simulator: `~/Library/Developer/CoreSimulator/Caches`(순수 캐시)와 `Devices/`(각 시뮬레이터의 설치 앱·앱 데이터, 보통 macOS 개발 공간에서 가장 큰 소비처) 둘 다 탐지한다. `Devices/`는 개발자가 의도적으로 seed한 상태(테스트 fixture·로그인 세션)를 담을 수 있어 순수 캐시가 아니라 **Docker Volume과 같은 성격**으로 본다 — 그래서 항상 `REVIEW`(자동 정리 대상 아님)로만 표시하고, 격리/숙청 경로에는 절대 넣지 않는다(2026-07-22 결정, §19.8이 "별도 결정 전까지 제외"로 남겨뒀던 항목). 구현상 `global-cache` 타입을 재사용하는데, 이는 캐시라서가 아니라 `DefaultRiskPolicy`에서 `global-cache`가 구조적으로 절대 SAFE에 도달하지 못해 이 user-data 디렉터리에 필요한 안전 속성과 정확히 일치하기 때문이다(새 `ResourceType`/migration 없음, version `simulator-devices`로 전용 simctl 안내 문구 선택). "unavailable"(정리해도 안전) 대 "available"(사용 중) 기기 구분은 `xcrun simctl list devices`가 필요해 실제 Simulator 설치된 머신에서 검증 가능해질 때까지 후속으로 남긴다. runtime 이미지(`/Library/Developer/CoreSimulator`, `~/Library`가 아닌 시스템 전역 경로)는 Homebrew Cellar와 같은 이유로 여전히 제외한다.

전역 cache는 모두 `global-cache`/`REVIEW`이며 자동 plan/clean 대상이 아니다(§19.7과 동일 정책). 도구가 설치되지 않았거나 캐시 디렉터리가 없으면 빈 결과이고 scan issue를 만들지 않는다.

구현은 `internal/adapter/xcode`, `cocoapods`, `swiftpm`, `homebrew`, `simulator` package로 분리하고 공통 `cachepath` helper를 재사용한다. `OFFICIAL_CLEANUP_GUIDANCE` reason으로 다음 안내만 제공한다: Xcode DerivedData는 통째로 삭제 가능(`rm -rf` 또는 Xcode Settings > Locations), CocoaPods는 `pod cache clean --all`, SwiftPM은 `rm -rf ~/Library/Caches/org.swift.swiftpm`(또는 `swift package purge-cache`), Homebrew는 `brew cleanup`/`brew cleanup -s`, iOS Simulator 캐시는 자동 재생성되며 설치된 runtime/device 관리는 Xcode Settings > Platforms 또는 `xcrun simctl`을 안내한다.

**남은 범위**: Simulator runtime 이미지의 위험도 분류, 그리고 `Devices/`의 available/unavailable 기기 구분(simctl 필요)은 여전히 후속으로 남긴다. (`Devices/` 자체의 read-only REVIEW 탐지는 2026-07-22에 추가됐다 — 위 iOS Simulator 항목 참고.) (2026-07-21 갱신: symlink·APFS 별도 볼륨 경계는 실제 검증 완료 — §11 참고. macOS 시스템 경로 보호와 권한 오류 처리도 검증 완료 — §8 참고. Xcode/SwiftPM 프로젝트 타입 탐지와 의존성 그래프는 §19.9에서 구현됐다 — 더 이상 범위 밖이 아니다.)

> 같은 날 별도로: `.NET SDK` 탐지(`dotnet-sdk`, `internal/adapter/dotnet`)를 macOS/Linux에서도
> 풀었다. `dotnet` CLI 자체는 원래 크로스플랫폼이라 `adapter.RequireWindows` 게이트는 기술적
> 한계가 아니라 "Windows 우선" 제품 범위 결정이었을 뿐이었다. Windows의 고정 설치 경로
> (`C:\Program Files\dotnet\dotnet.exe`)와 os.Stat 기반 존재 확인은 그대로 두고, 비Windows는
> npm/pnpm/Homebrew와 같은 `exec.LookPath("dotnet")` 방식으로 분기했다 — 이 개발 환경엔
> dotnet이 설치돼 있지 않아 mock 기반 단위 테스트로만 검증했다.

### 19.9 Xcode/SwiftPM 프로젝트 (`IMPLEMENTED`, 2026-07-22)

§19.8이 남겨둔 "Xcode/SwiftPM 프로젝트 타입 탐지와 의존성 그래프"를 구현한다. §4의 domain
model에 project type `xcode`(.xcodeproj)/`swiftpm`(Package.swift), workspace type
`xcode-workspace`(.xcworkspace), resource type `xcode-install`/`cocoapods-pods`를 추가했다 —
새 DB migration은 필요 없다(모두 문자열 enum 값 추가, column 추가 아님).

**프로젝트 탐지**

- `.xcodeproj`: 디렉터리 번들 자체가 manifest 역할(`<name>.xcodeproj/project.pbxproj`), 부모 디렉터리가 project root다. `internal/adapter/xcodeproj.Detector`. **번들 안에 `project.pbxproj`가 실제로 존재해야 프로젝트로 인정한다** — 이름만 `.xcodeproj`로 끝나는 빈/백업 디렉터리(`Backup.xcodeproj/` 등)는 `Detect`가 error를 반환해 `MALFORMED_MANIFEST` issue로 노출되고, 존재하지 않는 manifest가 프로젝트 ID나 evidence를 앵커링하지 않는다.
- `.xcworkspace`: `contents.xcworkspacedata`의 최상위 `<FileRef location="group:...">`만 `.xcodeproj` 멤버로 해석한다(`internal/adapter/xcodeproj.WorkspaceDetector`). 중첩된 `<Group>` 하위 멤버는 해석하지 않는다 — CocoaPods가 생성하는 일반적인 "앱 자신 + Pods.xcodeproj" 평면 구조는 커버하지만, Xcode UI에서 폴더로 정리한 대형 workspace는 일부 멤버를 놓칠 수 있다. 현재는 `group:`/`container:`/`self:` prefix를 모두 제거하고 workspace 부모 디렉터리에 join하는데, 이는 평면 구조에서만 옳다. 중첩 Group 지원을 추가하면 이 prefix들은 더 이상 동일하게 취급할 수 없어(`group:`은 상위 Group 기준) element별 base 경로 추적이 필요하다.
- `Package.swift`: `internal/adapter/swiftpm.Detector`. 첫 줄의 `// swift-tools-version:X.Y` 주석을 `ProjectProperty`(이름 `swift-tools-version`)로 실어 나른다. 단 이 값은 **현재 어떤 analyzer도 소비하지 않는다** — 아래 의존성 분석에서 설명하듯 Xcode 의존성 근거로 쓰지 않으며(Swift tools 호환 레벨이지 특정 Xcode 의존 선언이 아님), 향후 Swift-toolchain 리소스 모델을 위한 예약 필드로만 남겨둔다.

**프로젝트 소유 산출물** (project-owned artifact, OWNS edge)

| 산출물 | 조건 | 재생성 근거 |
|---|---|---|
| `Pods/` | 같은 디렉터리에 `Podfile`이 있어야 함 | `Podfile.lock` 있으면 DECLARED+Regenerable, 없으면 INFERRED+비-Regenerable |
| `.build/` | `Package.swift` 프로젝트 root | `Package.resolved` 있으면 DECLARED+Regenerable, 없으면 INFERRED+비-Regenerable |

둘 다 `internal/app.projectArtifactCleanupEvidence`(기존 node_modules/venv와 동일한 helper)로
ProjectOwned/ReparsePointFree/GitTrackedOriginalsAbsent를 검증하므로, 조건을 모두 만족하면
`DefaultRiskPolicy`가 그대로 `SAFE`를 낸다 — risk_policy.go에 새 특수 케이스를 추가하지
않았다. `.build`는 기존 `ResourceTypeBuildOutput`(bin/obj와 동일 타입)을 재사용하고, `Pods`는
전역 CocoaPods 캐시(§19.8, `global-cache`/`cocoapods-cache`)와 다른 리소스이므로 새 타입
`cocoapods-pods`를 썼다.

**활성 Xcode 설치** (`xcode-install`, `internal/adapter/xcode.InstallLister`)

`xcode-select -p`로 **현재 활성화된** 개발자 디렉터리를, `xcodebuild -version`으로 버전을 얻는다.
Command Line Tools만 설치되고 전체 Xcode.app이 없으면 `xcodebuild`가 실패하는데, 이 경우 빈
결과를 반환한다(실패가 아니라 "설치 안 됨"으로 취급) — Visual Studio의 `visual-studio`/`msbuild`
리소스와 같은 급의 시스템 리소스이지, Command Line Tools 자체를 추적 대상으로 삼지 않는다.
`SystemManaged`를 항상 true로 설정한다 — 일부 개발자는 sudo 없이 여러 Xcode 버전을 관리하려고
`~/Applications`처럼 사용자 홈 밑에 설치하기도 해서, `/Applications` 등 시스템 경로 기반
보호(§8)만으로는 못 잡는 경우를 방어한다.

**범위는 "활성 Xcode 하나"로 의도적으로 제한한다.** 한 머신에 Xcode.app·Xcode-beta.app·사용자
지정 위치의 여러 Xcode가 공존할 수 있는데, 이 어댑터는 `xcode-select`가 가리키는 것 하나만
보고한다 — 비활성 설치본의 디스크 사용량은 집계되지 않고, `xcode-select` 전환에 따라 리소스
목록이 달라진다. 이를 리소스 이름 `"Xcode (active)"`로 드러낸다. 설치본 전체 열거(mdfind/Spotlight로
`*.app` 후보를 찾고 각 bundle version metadata를 읽는 방식)는 후속 범위다.

**의존성 분석** (`internal/app.XcodeDependencyAnalyzer`)

`.xcodeproj` 프로젝트만 활성 `xcode-install` 리소스에 `REQUIRES`로 연결한다 — `.xcodeproj`
빌드는 실제로 전체 Xcode(`xcodebuild`)가 필요하므로 유효한 관계이며, MSBuild가 Windows
SDK/.NET SDK와 맺는 관계와 같다. 근거는 항상 `INFERRED`다("이 프로젝트 종류는 Xcode가 필요하고,
활성 설치본이 정확히 하나 있다"는 존재 매칭일 뿐, 버전 매칭 아님 — 순수 `.xcodeproj`엔 선언된
Xcode 버전이 없다). 감지된 활성 Xcode가 없으면 `UnverifiedScope`로 남기고 edge를 만들지 않는다.

**SwiftPM(`Package.swift`)은 이 edge에서 의도적으로 제외한다.** Swift 패키지는 `swift build`로
빌드되고 이는 Command Line Tools·standalone toolchain·Linux의 Swift 등 **어떤 Swift toolchain으로도**
가능하지, 특정 Xcode.app을 요구하지 않는다. 무조건 REQUIRES-Xcode edge를 만들면 `impact
xcode-install:<version>`이 "Xcode 삭제 시 이 SwiftPM 프로젝트가 깨진다"고 **잘못** 보고한다.
`swift-tools-version`도 Swift tools 호환 레벨이지 특정 Xcode 의존 선언이 아니다. Swift-toolchain
리소스를 별도로 모델링하기 전까지 SwiftPM은 edge 대신 `UnverifiedScope`(관계는 존재하나 분석되지
않음)를 기록한다 — 관계를 조용히 누락하지 않으면서 거짓 impact도 만들지 않는 절충이다.

`explain`/`impact`의 RUN/BUILD/DEBUG 판단은 `internal/app.ImpactService.Assess`가
`RelationRequires` edge만 보고 이미 완전히 generic하게 처리하므로 추가 코드가 필요 없었다.

**남은 범위**: (1) SwiftPM의 toolchain 의존성을 표현할 `swift-toolchain`/`swift-sdk` 리소스
모델링(위 UnverifiedScope를 실제 edge로 승격). (2) DerivedData를 프로젝트별로 나눠 소유(OWNS)로
연결하는 것(Xcode가 `info.plist`의 `WorkspacePath`로 매핑) — §19.8의 전역 aggregate 캐시와 이중
계산될 위험 때문에 보류. (3) 활성이 아닌 Xcode 설치본 전체 열거. 실제 전체 Xcode.app이 설치된
머신에서의 검증도 아직 못 했다(이 개발 환경은 Command Line Tools만 있음) — unit test와
fixture(`testdata/xcode/`, `testdata/swiftpm/`) 기반 e2e로만 검증했다.

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

### 20.2 Confidence (`CONFIRMED`, 2026-07-20)

EvidenceKind별 기본 점수를 `internal/domain.DefaultConfidence`로 확정했다. 모든 어댑터는 자체 스케일을 만들지 않고 이 맵을 그대로 참조한다.

```text
RESOLVED  90
OBSERVED  85
DECLARED  75
PINNED    60
INFERRED  40
UNKNOWN   10
```

확정 전에는 `internal/adapter/msbuild/resolve.go`만 이 표와 같은 숫자를 우연히 쓰고 있었고, `internal/adapter/node`(60/35/30)와 `internal/adapter/msbuild/artifacts.go`(30)는 서로 다른 placeholder 스케일을 썼다. 이번에 세 곳 모두 `domain.DefaultConfidence`를 참조하도록 통일했다:

| 어댑터 | 근거 | EvidenceKind |
|---|---|---|
| `msbuild/resolve.go` | SDK 버전 매칭 (`MatchWindowsSDK`/`MatchDotNetSDK`) | 매칭 방식에 따라 RESOLVED/DECLARED |
| `node` | `package.json` + lockfile 존재 (재생성 가능성 선언됨) | DECLARED |
| `node` | `node_modules` 존재하지만 lockfile 없음 | INFERRED |
| `node`, `msbuild/artifacts.go` | `dist`/`.next`/`build`/`out`/`bin`/`obj` 등 디렉터리 이름만 일치, 설정 확인 안 함 (§19.3) | INFERRED |

복수 Evidence 가산과 UnverifiedScope 감점 규칙은 이번 확정 범위에 포함하지 않았다 — 지금 어떤 리소스도 두 개 이상의 Evidence를 동시에 갖는 경우가 없어서(리소스당 매칭 시도 1회) 합산할 대상 자체가 없다. 실제로 여러 Evidence가 쌓이는 시나리오가 생기면 그때 다시 결정한다.

추후 점수를 변경하면 계약 변경으로 처리하고 Evidence/impact 회귀 테스트를 함께 갱신한다.

```text
기본 점수       = 가장 강한 Evidence
서로 다른 보조 근거 = 제한된 가산  ── 미구현 (DECISION_REQUIRED로 남음)
UnverifiedScope = 항목별 감점    ── 미구현 (DECISION_REQUIRED로 남음)
최종 범위       = 0..100
```

Confidence가 높다는 사실은 Risk가 SAFE라는 의미가 아니다.

### 20.3 Risk 중앙 정책 (`CONFIRMED`)

adapter는 사실과 Evidence만 반환하고 application의 `RiskPolicy`가 판정한다.

```go
type RiskPolicy interface {
    Classify(ResourceContext) RiskAssessment
}

type CleanupEvidence struct {
    ProjectOwned              bool
    KnownOutputPath           bool
    ReparsePointFree          bool
    GitTrackedOriginalsAbsent bool
}

type RiskAssessment struct {
    Level      RiskLevel
    Confidence ConfidenceProfile
    Blockers   []RiskReason
    Warnings   []RiskReason
    Safeguards []RiskReason
    Unknowns   []RiskReason
}
```

- 모든 조건을 만족하고 재생성 가능: `SAFE`
- 하나라도 미검증: `REVIEW`
- policy에 전달된 critical unknown: `REVIEW`
- system-managed, protected path 또는 denylist: `BLOCKED`
- 폴더 이름만으로 SAFE가 되지 않는다.
- scan 시 Node/MSBuild detector는 ownership/output 사실을 제공한다.
- 실제 `clean --execute`는 reparse/Git/경로/크기/mtime을 다시 검사한다.

## 8. allowlist, denylist와 link (`IMPLEMENTED`)

allowlist basename:

```text
node_modules, bin, obj, build, dist, .next, out, Debug, Release
.venv, venv, env, __pycache__, .pytest_cache, .mypy_cache, *.egg-info(suffix)
```

이름 일치 외에 project root 내부, explicit `OWNS`, SAFE, regenerable, 비보호 경로, 비-reparse, Git tracked 원본 없음, 실행 직전 snapshot 일치가 모두 필요하다.

Python 항목은 §19.4 결정 6을 따른다: 캐시(`__pycache__` 등)는 항상 이름
일치만으로 충분하지만, venv(`.venv`/`venv`/`env`)는 이름 일치와 별개로
`Regenerable`이 lockfile 등급 `PINNED` 이상일 때만 true가 되므로 실질적으로
더 엄격하다. `*.egg-info`는 패키지 이름이 접두사라 고정 basename이 아닌
suffix로 판정한다(`internal/safety.isAllowedArtifactName`).

conda 환경(`conda-env` 타입)은 로컬 prefix 환경이라도 이 allowlist에
포함하지 않는다 — §19.5 결정 4가 conda 환경을 cleanup 경로 밖에 두기 때문에,
해당 리소스는 `CleanupEvidence`가 항상 zero-value로 남아 애초에 SAFE
판정을 받지 못한다.

denylist와 모든 하위 경로:

```text
Windows/Program Files 계열
사용자 문서
.git
.env
인증서와 key
Libra DB
.libra-quarantine
알 수 없는 대용량 경로
conda 환경 (전역 named 환경, 로컬 prefix 환경 모두 — §19.5 결정 4)
```

```text
scan    link 자체만 기록하고 target을 따라가지 않음
clean   symlink/junction/reparse point BLOCKED
restore manifest에 기록된 일반 directory만 처리
```

OS 기본 보호 root는 `PathClassifier`가 판정한다. 사용자 문서·비밀 파일의 내용 기반 탐지는 아직 `PLANNED`이며, 자동 SAFE가 아닌 항목은 기본적으로 선택되지 않는다.

Windows는 `WINDIR`/`ProgramFiles`/`ProgramFiles(x86)`/`ProgramData` 환경변수 기반 root를 쓴다(`internal/safety/roots_windows.go`). macOS는 2026-07-21까지 이 목록이 비어 있었다 — MVP 범위 결정으로 기록돼 있었을 뿐 버그는 아니었지만, 실제로는 잘못된 `project_roots` 설정이 `/System`·`/Library`·`/usr` 같은 시스템 경로를 가리켜도 `BLOCKED`가 아니라 `REVIEW`로만 표시되는 라벨링 공백이었다. `internal/safety/roots_other.go`에 `/System`, `/Library`, `/usr`, `/bin`, `/sbin`, `/Applications`를 darwin 전용으로 추가해 닫았다 — SIP가 이미 쓰기 자체를 막지만, 이건 `explain`/`plan` 표시가 실제 위험도를 정확히 반영하도록 만드는 라벨링 계층이다. `~/Library`는 의도적으로 제외한다 — §19.8의 macOS 개발 캐시 5종(Xcode/CocoaPods/SwiftPM/Homebrew/Simulator)이 전부 그 밑에서 리소스를 보고하므로, 포함하면 전부 `BLOCKED`로 잘못 표시된다. Linux 시스템 경로는 실제 Linux 환경에서 검증 전이라 여전히 `nil`(미분류)로 남겨뒀다.

실제 macOS 권한 오류 처리도 검증했다: fixture 디렉터리를 `chmod 000`으로 막고 scan한 결과 `ACCESS_DENIED` structured issue로 기록되고 나머지 프로젝트(`readable-project`)는 정상적으로 계속 발견됐다 — 한 경로의 권한 오류가 전체 스캔을 중단시키지 않는다는 계약(§5)이 macOS 실제 권한 오류에서도 유지됨을 확인했다. TCC(Full Disk Access 등 macOS 고유 동의 프롬프트)로 인한 거부는 OS 레벨에서 동일한 `EPERM`/`EACCES`로 나타나므로 같은 경로로 처리되지만, 실제 TCC 프롬프트를 토글해 가며 검증하지는 않았다 — 필요하면 다음 범위로 남긴다.

## 9. Plan (`IMPLEMENTED`)

선택 순서:

1. BLOCKED 제외
2. SAFE이면서 Dependency ≥ 80, CleanupSafety ≥ 90, ScanCoverage ≥ 80인 항목만 자동 선택
3. 호환용 요약 Confidence 내림차순
4. ReclaimableSize 내림차순
5. stable Resource ID 오름차순
6. target 이상이면 중단

실행 단위는 Resource다. `OWNS` edge가 없는 resource는 자동 선택하지 않는다. 목표를 채우지 못해도 plan을 저장하고 `INSUFFICIENT_CANDIDATES`와 실제 선택 용량을 출력한다.
임계값을 통과하지 못한 SAFE resource는 자동 선택하지 않고 REVIEW 목록으로 보낸다.
현재 resource 관측은 Dependency와 ScanCoverage에 임계값인 80을 호환 baseline으로 기록한다.
후속 pipeline 연결에서 `UnverifiedScope` 또는 root 실패가 확인되면 이 값을 낮추고 critical
unknown을 전달해 `REVIEW`로 재분류한다 (`CONFIRMED`).

PlanItem snapshot:

- ResourceID, NormalizedPath, ExpectedType
- ExpectedSize, ExpectedModifiedTime
- RiskAtPlanning, ConfidenceAtPlanning
- OwnerProjectID, ScanID, RegenerationCommand

## 10. Clean과 실행 직전 재검증 (`IMPLEMENTED`)

```text
libra clean --plan <id>                 dry-run
libra clean --plan <id> --execute       실제 격리 + 확인
libra clean --plan <id> --execute --yes 실제 격리 + 확인 생략
```

`--yes`는 확인 생략만 의미한다. 실행 활성화는 반드시 `--execute`가 담당한다.

실행 직전 확인:

- normalized path와 resource identity/type
- 실제 전체 크기와 최종 수정 시각
- explicit owner project와 root containment
- 현재 Risk SAFE와 Regenerable
- allowlist와 protected path
- Git tracked file 존재 여부
- symlink/junction/reparse 여부

불일치 item은 `SKIPPED`하고 나머지는 계속한다. 한 개 이상 이동하고 일부가 실패/skip되면 `PARTIALLY_QUARANTINED`, 전부 실패하면 `FAILED`다.

## 11. Quarantine와 manifest (`IMPLEMENTED`)

하나의 논리 transaction ID를 쓰되 volume마다 root를 둔다.

```text
<volume>\.libra-quarantine\<transaction-id>\
├─ manifest.json
└─ items\<stable-item-name>
```

비Windows에서는 같은 filesystem을 유지하도록 원본 parent 아래에 둔다. 실제 처리 순서:

1. schema version 1 disk manifest 원자 기록
2. `os.Rename`으로 같은 volume 이동
3. item 상태를 반영해 manifest 원자 갱신
4. 최종 transaction과 items를 DB에 기록

DB 기록 실패 메시지는 disk manifest로 복구해야 함을 명시한다. manifest 생성 실패 시 이동을 시작하지 않는다. Windows hidden attribute와 ACL의 별도 복제는 `PLANNED`; rename이 보존하는 기존 metadata를 사용한다.

> 검증(2026-07-21, macOS): "비Windows에서는 원본 parent 아래에 둔다"는 이 계약을 실제
> 별도 APFS 볼륨(`hdiutil`로 만든 scratch volume, 별도 device node로 확인)에서
> `scan → plan → clean --execute → restore` 전체 사이클로 검증했다. `node_modules`
> 전체가 해당 볼륨 안에 있었으므로 `.libra-quarantine`도 같은 볼륨의 원본 parent
> 밑에 생겨 `os.Rename`이 볼륨 경계를 넘지 않았고, cross-device(EXDEV) 실패 없이
> MOVED/RESTORED가 정상 완료됐다. 이 배치 보장 자체(volume root가 아니라 원본
> parent 밑)는 `internal/safety/quarantine_nonwindows_test.go`의
> `TestQuarantineEngineDefaultRootStaysUnderOriginalParent`로 고정했다 — 기존
> quarantine 테스트 두 개는 모두 `RootForPath`를 override해서 실제 프로덕션
> `root()` 배치 로직을 한 번도 실행하지 않았던 것을 이번에 발견해 추가했다.
> 남은 범위: 프로젝트 자체가 볼륨 경계에 걸쳐 있는 경우(예: 프로젝트 루트는 내장
> 디스크, 그 안의 산출물만 별도 마운트) 등 더 특이한 배치는 여전히 미검증이다.

## 12. Transaction과 Restore (`IMPLEMENTED`)

Transaction status:

```text
PLANNED, RUNNING, QUARANTINED, PARTIALLY_QUARANTINED,
RESTORED, PARTIALLY_RESTORED, PURGED, PARTIALLY_PURGED, FAILED
```

Item status:

```text
PENDING, MOVED, SKIPPED, FAILED, RESTORED, PURGED
```

```text
libra transactions
libra restore --transaction <id>
```

restore 규칙:

- 원본 경로가 존재하면 overwrite/merge하지 않고 해당 item만 SKIPPED
- 나머지 item은 계속 복구
- quarantine item이 없거나 rename이 실패하면 FAILED
- 매 item 후 manifest를 갱신하고 마지막에 DB 상태를 갱신

`quarantine_days`는 purge 가능 표시 기준일 뿐 자동 삭제 시점이 아니다. `libra purge --transaction <id>`는 기본 dry-run이며 manifest identity, item 경로, link/reparse 여부를 다시 검증한다. `--execute`와 사용자 확인(또는 전역 `--yes`)이 함께 있을 때만 영구 삭제한다. 일부 item만 삭제되면 `PARTIALLY_PURGED`, 전부 삭제되면 `PURGED`다 (`IMPLEMENTED`).

## 13. CLI와 출력 현황

| 명령 | 상태 |
|---|---|
| init, scan, summary, projects, resources, issues | IMPLEMENTED |
| explain, impact | IMPLEMENTED |
| plan | IMPLEMENTED |
| clean dry-run/execute | IMPLEMENTED |
| transactions, restore | IMPLEMENTED |
| purge, export, daemon, events | IMPLEMENTED |

`libra issues`는 기본적으로 가장 최근에 시작한 scan의 경고·오류를 조회한다. `--scan <id>`로
과거 scan을 지정하고 `--code`와 `--severity`를 함께 또는 각각 적용할 수 있다. 텍스트와
`--json` 모두 `scan_id`와 issue의 code/phase/adapter/path/operation/severity/message를 제공한다
(`IMPLEMENTED`, issue #47). `scan --strict`는 이 계약에 포함하지 않는다.

모든 명령의 `--json`이 아래 공통 envelope로 나간다 (`IMPLEMENTED`, issue #42/#59). `output.New(w, jsonOutput, command)`이 command 이름을 받고, `Printer.Print`/`PrintEnvelope`가 감싼다.

```json
{"command":"clean","schema_version":1,"outcome":"PARTIAL","data":{},"issues":[],"unverified":[]}
```

`init`, `events`, `export`, `daemon start/status/stop`도 같은 envelope를 사용한다. 하위 명령의 `command`는 `daemon status`처럼 전체 경로를 기록한다. 단, `export --format json`은 다른 도구로 전달하는 원본 report 포맷을 유지하고, 전역 `--json export`를 지정했을 때만 CLI envelope를 적용한다. `--json --format markdown` 조합은 거부한다.

- **`status` → `outcome`으로 개명**: 최초 초안은 `status`였는데, `PlanView.Status`(READY/INSUFFICIENT_CANDIDATES) 등 `data` 하위 필드도 이미 `status`라는 이름을 쓰고 있어 JSON depth만 다르고 이름이 같은 두 개념이 헷갈릴 수 있었다(`jq .status`가 어느 쪽을 가리키는지 불명확). envelope 레벨은 `outcome`(`SUCCESS`/`PARTIAL`/`FAILED`)으로 분리했다.
- **`issues`/`unverified` 실제 배선**: `scan`/`issues`/`clean`(dry-run 포함)/`purge`/`restore` 5개 명령만 채운다 — 이 명령들만 "이번 실행 자체가 부분적으로만 성공했다"는 개념이 구조적으로 존재한다(예: scan이 일부 경로에서 권한 오류를 만났거나, clean이 일부 item만 이동함). 나머지(`projects`/`resources`/`summary`/`explain`/`impact`/`transactions`/`plan`)는 DB를 읽기만 하거나(전자) 매 요청이 전부 성공하거나 에러로 죽는 이분법이라, `issues: []`가 스텁이 아니라 정확한 값이다. `plan`만 예외로 `outcome`은 `INSUFFICIENT_CANDIDATES`일 때 `PARTIAL`로 매핑하되(목표 미달도 "완료했지만 부족" 케이스라서), `issues` 배선은 이번 스코프에 넣지 않았다(SAFE/REVIEW/BLOCKED가 이미 후보별 `risk_reasons`를 따로 들고 있어, envelope 레벨 issues와 어떻게 합칠지는 별도 결정 필요).
- `Unverified`(app.UnverifiedScope)는 scan 실행 중에만 존재하고 DB에 영속화되지 않아, scan 이외 명령은 구조적으로 접근 불가 — scan만 채운다.

Exit code 목표:

| 코드 | 의미 | 상태 |
|---:|---|---|
| 0 | 성공 또는 dry-run 성공 | IMPLEMENTED |
| 1 | 인자·일반 명령 오류 | IMPLEMENTED |
| 2 | target/plan/transaction 없음 | IMPLEMENTED |
| 3 | DB·파일시스템 내부 오류 | IMPLEMENTED |
| 4 | safety 차단 | IMPLEMENTED |
| 5 | 부분 clean/restore/purge | IMPLEMENTED |
| 130 | 사용자 취소 | IMPLEMENTED |

`cmd.ExitCode`가 wrapped sentinel error와 부분 cleanup transaction 상태를 위 코드로 변환한다 (`IMPLEMENTED`, issue #62).

## 14. 테스트와 완료 조건

필수 자동 검증:

```bash
gofmt -l .
go vet ./...
go test ./... -count=1
go build ./...
```

cleanup fixture는 임시 디렉터리만 사용하고 다음을 검증한다.

- dry-run에서 변경 없음
- manifest가 이동 전에 존재
- 실제 이동 후 원본 부재와 quarantine 존재
- restore 원위치 복구
- 원본 충돌 시 overwrite 금지
- symlink/reparse/protected/Git tracked/변경 snapshot 차단
- item 실패가 다른 item을 막지 않음
- DB transaction/item 상태 일치

## 15. 남은 작업

| 우선순위 | 항목 |
|---:|---|
| 1 | ~~typed CLI error와 exit code 2/3/4/5/130 연결~~ — `IMPLEMENTED` (issue #62) |
| 2 | ~~모든 명령 공통 JSON envelope migration~~ — `IMPLEMENTED` (issue #42/#59) |
| 3 | Windows 실제 volume에서 junction, ACL, hidden attribute 통합 테스트 |
| 5 | incremental scan snapshot과 STALE 전환 |
| 6 | daemon OS-native watcher/lock 완성 |
| 7 | ~~`daemon status --json`을 공통 envelope로 편입~~ — `daemon start/status/stop` 모두 `IMPLEMENTED` (issue #42/#59) |

## 16. 변경 관리

계약을 바꿀 때 PR에 다음을 포함한다.

1. 변경 전후 의미와 호환성
2. domain/DB/CLI 영향
3. migration과 rollback 또는 복구 방법
4. fixture·unit·integration 테스트
5. README·일정·이 문서 갱신
6. 공동 계약은 2명, cleanup/safety는 Windows A 포함 2명 승인

`main` 병합 전 문서가 현재 구현을 설명하는지 처음부터 끝까지 다시 확인한다.
