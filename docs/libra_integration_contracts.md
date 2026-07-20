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
- 지원 project type: `msbuild-cpp`, `msbuild-dotnet`, `node`, `git`
- 상태: `ACTIVE`, `STALE`, `ARCHIVED`, `UNKNOWN`

### Resource

지원 type은 `windows-sdk`, `netfx-sdk`, `visual-studio`, `msbuild`, `dotnet-sdk`, `node-modules`, `build-output`, `global-cache`, `docker-cache`다.

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
| INFERRED | 40 |
| UNKNOWN | 10 |

Confidence는 분석 coverage이지 실제 확률이나 cleanup 안전도를 뜻하지 않는다. 복수 근거 가산과 Unverified 감점은 `PLANNED`다.

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
- scan record가 완료되기 전에 project/resource/dependency 결과를 저장한다.

Incremental snapshot 전환, 미발견 project의 자동 STALE 처리와 scan 동시 실행 lock은 `PLANNED`다.

## 6. Repository와 transaction (`IMPLEMENTED`)

- Project: scan 단위 `UpsertObserved`, ID/manifest 조회, 목록
- Resource: `Upsert`, ID/type/전체 조회
- Dependency: edge와 Evidence를 하나의 DB transaction으로 upsert
- CleanupPlan: plan과 모든 item snapshot을 하나의 transaction으로 생성
- CleanupTransaction: transaction/item 상태를 함께 생성·갱신·조회
- migration은 번호 순서로 적용하고 `schema_migrations`에 기록한다.

현재 schema migration은 `001`부터 `008`까지다. 기존 row를 파괴하지 않고 column/table을 추가한다.

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
type CleanupEvidence struct {
    ProjectOwned              bool
    KnownOutputPath           bool
    ReparsePointFree          bool
    GitTrackedOriginalsAbsent bool
}
```

- 모든 조건을 만족하고 재생성 가능: `SAFE`
- 하나라도 미검증: `REVIEW`
- system-managed, protected path 또는 denylist: `BLOCKED`
- 폴더 이름만으로 SAFE가 되지 않는다.
- scan 시 Node/MSBuild detector는 ownership/output 사실을 제공한다.
- 실제 `clean --execute`는 reparse/Git/경로/크기/mtime을 다시 검사한다.

## 8. allowlist, denylist와 link (`IMPLEMENTED`)

allowlist basename:

```text
node_modules, bin, obj, build, dist, .next, out, Debug, Release
```

이름 일치 외에 project root 내부, explicit `OWNS`, SAFE, regenerable, 비보호 경로, 비-reparse, Git tracked 원본 없음, 실행 직전 snapshot 일치가 모두 필요하다.

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
```

```text
scan    link 자체만 기록하고 target을 따라가지 않음
clean   symlink/junction/reparse point BLOCKED
restore manifest에 기록된 일반 directory만 처리
```

OS 기본 보호 root는 `PathClassifier`가 판정한다. 사용자 문서·비밀 파일의 내용 기반 탐지는 아직 `PLANNED`이며, 자동 SAFE가 아닌 항목은 기본적으로 선택되지 않는다.

## 9. Plan (`IMPLEMENTED`)

선택 순서:

1. BLOCKED 제외
2. SAFE만 자동 선택
3. Confidence 내림차순
4. ReclaimableSize 내림차순
5. stable Resource ID 오름차순
6. target 이상이면 중단

실행 단위는 Resource다. `OWNS` edge가 없는 resource는 자동 선택하지 않는다. 목표를 채우지 못해도 plan을 저장하고 `INSUFFICIENT_CANDIDATES`와 실제 선택 용량을 출력한다.

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

## 12. Transaction과 Restore (`IMPLEMENTED`)

Transaction status:

```text
PLANNED, RUNNING, QUARANTINED, PARTIALLY_QUARANTINED,
RESTORED, PARTIALLY_RESTORED, PURGED, FAILED
```

Item status:

```text
PENDING, MOVED, SKIPPED, FAILED, RESTORED
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

`quarantine_days`는 purge 가능 표시 기준일 뿐 자동 삭제 시점이 아니다. 명시적 dry-run purge 명령과 `PURGED` 실행은 `PLANNED`다.

## 13. CLI와 출력 현황

| 명령 | 상태 |
|---|---|
| init, scan, summary, projects, resources | IMPLEMENTED |
| explain, impact | IMPLEMENTED |
| plan | IMPLEMENTED |
| clean dry-run/execute | IMPLEMENTED |
| transactions, restore | IMPLEMENTED |
| purge, export, daemon | PLANNED |

현재 `--json`은 각 command view를 JSON으로 직렬화한다. 아래 공통 envelope는 합의됐지만 기존 명령 전체 migration이 끝나지 않아 `CONFIRMED`다.

```json
{"command":"clean","status":"PARTIAL","data":{},"issues":[],"unverified":[]}
```

Exit code 목표:

| 코드 | 의미 | 상태 |
|---:|---|---|
| 0 | 성공 또는 dry-run 성공 | IMPLEMENTED |
| 1 | 인자·일반 명령 오류 | IMPLEMENTED |
| 2 | target/plan/transaction 없음 | CONFIRMED |
| 3 | DB·파일시스템 내부 오류 | CONFIRMED |
| 4 | safety 차단 | CONFIRMED |
| 5 | 부분 clean/restore | CONFIRMED |
| 130 | 사용자 취소 | CONFIRMED |

Cobra error를 위 코드로 변환하는 최상위 typed error adapter는 아직 `PLANNED`다.

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
| 1 | typed CLI error와 exit code 2/3/4/5/130 연결 |
| 2 | 모든 명령 공통 JSON envelope migration |
| 3 | Windows 실제 volume에서 junction, ACL, hidden attribute 통합 테스트 |
| 4 | explicit `purge` dry-run/execute |
| 5 | incremental scan snapshot과 STALE 전환 |
| 6 | daemon lock/event 병합/export schema |

## 16. 변경 관리

계약을 바꿀 때 PR에 다음을 포함한다.

1. 변경 전후 의미와 호환성
2. domain/DB/CLI 영향
3. migration과 rollback 또는 복구 방법
4. fixture·unit·integration 테스트
5. README·일정·이 문서 갱신
6. 공동 계약은 2명, cleanup/safety는 Windows A 포함 2명 승인

`main` 병합 전 문서가 현재 구현을 설명하는지 처음부터 끝까지 다시 확인한다.
