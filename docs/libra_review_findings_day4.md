# Day 4 코드 리뷰 준비: 협업 원칙 · 계약 · 구조 점검

> 작성: Mac C, 2026-07-20
> 목적: 실제 팀 코드 리뷰 세션 전에, `docs/libra_collaboration_rules.md`의 규칙과
> `docs/libra_integration_contracts.md`의 계약 대비 현재 코드/git 히스토리를
> 점검해서 문제 위치와 수정 제안을 미리 정리한다.
> 방법: git 히스토리, GitHub PR/이슈 API, 코드를 직접 읽어서 확인했다. 사람 사이의
> 대화(Slack 등)로만 합의된 내용은 git/GitHub에 흔적이 없으면 "확인 불가"로 남겼다.
> **이 문서는 결정 기록이 아니라 점검 결과다** — 각 항목은 담당자 확인과 팀 논의가
> 필요하고, 여기 적힌 "제안"은 시작점이지 최종 결론이 아니다.

---

## 요약

| # | 항목 | 심각도 | 담당 영역 |
|---|---|---|---|
| 1 | PR 리뷰가 사실상 한 번도 완료된 적 없음 | 높음 (프로세스) | 팀 전체 |
| 2 | `main`에 PR 없이 직접 push된 커밋 11개 이상 | 높음 (프로세스) | Windows B |
| 3 | 커밋 메시지 컨벤션 미준수 다수 | 중간 | 주로 Windows B |
| 4 | `DefaultRiskPolicy`가 계약(§20.3)과 달리 SAFE를 절대 반환하지 않음 | 중간 (계약 위배) | Windows A |
| 5 | `cmd` 계층이 명령마다 다른 구조를 씀 (application service 통과 여부) | 낮음 (구조 일관성) | Mac C 포함 전체 |
| 6 | `DependencyAnalyzer`가 scan에 연결되지 않음 | 이미 issue #22로 추적 중 | Windows B |
| 7 | `ScanService`(구 스캔 파이프라인)가 프로덕션에서 안 쓰이는 죽은 코드로 보임 | 낮음 (정리) | Windows A |
| 8 | `cmd/projects.go`의 `--type` 필터만 대소문자 구분 (다른 필터는 무시) | 낮음 (일관성) | Mac C |

1~3은 "어떻게 협업하는가"의 문제, 4~8은 "코드가 우리가 합의한 문서와 실제로 일치하는가 / 정리·일관성이 필요한가"의 문제로 나눴다.

---

## 1. PR 리뷰가 사실상 한 번도 완료된 적 없음

### 증거

`gh pr view <N> --json reviewRequests,reviews`로 당시 병합된 PR 18개를 전부 확인했다.

```
번호  작성자          리뷰 요청  실제 리뷰
#1    hyun020215      0         0
#2    hyun020215      0         0
#3    lunar-yoobin     0         0
#4    hyun020215      0         0
#5    hyun020215      0         0
#6    hyun020215      0         0
#7    lunar-yoobin     0         0
#9    lunar-yoobin     0         0
#11   lunar-yoobin     0         0
#12   hyun020215      0         0
#13   lunar-yoobin     1         0
#14   lunar-yoobin     0         0
#15   lunar-yoobin     1         0
#16   lunar-yoobin     0         0
#17   lunar-yoobin     0         0
#18   lunar-yoobin     0         0
#19   lunar-yoobin     1         0
#20   lunar-yoobin     0         0
```

**18개 PR 중 18개 모두 실제로 승인/리뷰된 적이 없다.** 리뷰 요청조차 3번(#13/#15/#19, 전부 Mac C 작성)뿐이었고, 그마저도 리뷰가 달리지 않은 채 병합됐다.

특히 `docs/libra_collaboration_rules.md` §6이 명시적으로 요구하는 경우들도 확인 안 됨:
- PR #6 "Fix adapter and resource handling for **Windows features**" (hyun020215 작성, 21개 파일 변경, `internal/adapter/windowsdk`/`msbuild`/`dotnet` 포함) → §6 "Windows 전용 탐지: Windows 팀원 1명 이상" 필요하지만 리뷰 요청 0.
- PR #12 "Implement project and workspace identities **with orchestration contracts**" (23개 파일, `internal/domain`/`internal/app` 등 공동 소유 영역 포함) → §6 "DB schema 또는 domain model: 2명 또는 팀 합의" 필요하지만 리뷰 요청 0.

### 왜 문제인가

이건 누구 한 명의 실수가 아니라 팀 전체가 "PR은 열지만 리뷰는 생략하고 바로 머지"하는 패턴으로 굳어진 것으로 보인다. `docs/libra_collaboration_rules.md` §13 "AI 사용 규칙"의 "AI가 생성한 코드는 사람이 이해하고 검증한 뒤 병합한다"도 이 프로세스가 실제로 돌지 않으면 지켜질 수 없다.

### 제안

- 지금부터 팀 코드 리뷰 세션을 실제로 진행 (사용자가 요청한 이 작업 자체가 그 시작점).
- 최소한 §6 표에 명시된 경우(DB/domain 변경, Windows 전용 탐지, cleanup 코드)만이라도 리뷰를 강제.
- 검토 당시 열려 있던 PR #23/#24/#25부터 실제 리뷰 절차를 적용하는 걸 제안.

---

## 2. `main`에 PR 없이 직접 push된 커밋

### 증거

`git log --merges`로 `main` 히스토리를 보면 "Merge pull request #N" 형태가 아닌 병합이 여러 개 있다. 그중 `main`에 직접 반영된 것들을 `gh api repos/.../commits/{sha}/pulls`로 확인하면(어느 PR에도 속하지 않으면 `[]`가 나온다):

```
$ gh api repos/madcamp-official/26s-w3-c2-01/commits/d5fe535/pulls
[]
$ gh api repos/madcamp-official/26s-w3-c2-01/commits/a7449fb/pulls
[]
```

아래 커밋들 전부 `[]` (어떤 PR에도 속하지 않음), 전부 Windows B(`Jaeyun-18`) 작성, 전부 `git merge` 형태로 `main`에 바로 올라감:

```
36cd6ac Merge branch 'day4'         (parent: main tip 8b0ff1c + d5fe535)
5025acb Merge branch 'day4'         (parent: main tip d645b59 + a7449fb)
436de2a Merge branch 'day4'
088d7b3 Merge Day 4 dependency graph work
d5fe535 issue 21 해결
a7449fb impact 및 unverified 형태 구성
1952d73 day4 중간
b55727e CI 수정
43e04fa mac test 문제 발생 해결
8550bbd scanner.entry 반영
423a439 계약 내용 바탕 수정
8a020f2 계약 사항 기준 id 중복 계산 형태 수정
8824aef day3 논의 사항 수정
ed29c3b day3 완료
863621b day2 완료
```

`main` 브랜치에 branch protection도 걸려 있지 않다 (`gh api repos/.../branches/main/protection` → 404 Not Found) — 그래서 기술적으로 직접 push가 막히지 않는다.

### 왜 문제인가

`docs/libra_collaboration_rules.md` §3 "main에 직접 push하지 않는다. 긴급 수정도 PR을 거친다"를 정면으로 위배한다. 결과적으로 `internal/domain/impact.go`, `internal/app/impact_service.go`, `internal/domain/unverified.go` 같은 **공동 소유 파일**(`internal/domain/*`, `internal/app/*`)이 리뷰 없이, PR 기록도 없이 `main`에 들어갔다 — §26.2 "domain field 추가·삭제"는 팀 합의가 필요한 변경인데 그 흔적이 GitHub에 전혀 없다.

### 제안

- `main` branch protection 설정 (PR 없이 직접 push 금지, 최소 1명 리뷰 요구) — 이게 근본적인 재발 방지책.
- 이미 들어간 코드를 되돌릴 필요는 없어 보인다(동작하고 테스트도 있음). 다만 `internal/domain/impact.go`/`internal/app/impact_service.go`가 실제로 팀이 원하는 설계인지 지금이라도 리뷰하고, 필요하면 후속 PR로 조정.

---

## 3. 커밋 메시지 컨벤션 미준수

### 증거

`docs/libra_collaboration_rules.md` §4가 요구하는 `<type>(<scope>): <내용>` 형식을 따르지 않는 커밋들 (전부 위 2번 항목의 직접-push 커밋과 겹친다):

```
gitignore 수정
issue 21 해결
impact 및 unverified 형태 구성
day4 중간
CI 수정
mac test 문제 발생 해결
scanner.entry 반영
계약 내용 바탕 수정
계약 사항 기준 id 중복 계산 형태 수정
day3 논의 사항 수정
day3 완료
day2 완료
day1 진행사항 merge
테스트 구성
adapter 인터페이스 정리
domain 모델 정의
Initial commit
```

PR을 거쳐 squash-merge된 커밋들(예: `feat(cmd): ...`, `fix(msbuild): ...`)은 전부 컨벤션을 잘 따르고 있다 — PR 제목이 커밋 메시지가 되는 §16 규칙 덕분이다. 즉 **PR을 거치면 컨벤션이 자연히 지켜지고, PR을 안 거치면 안 지켜진다** — 2번 항목과 사실상 같은 원인.

### 제안

2번(직접 push 금지)이 해결되면 이 항목은 자동으로 같이 해결된다. 별도 조치 불필요.

---

## 4. `DefaultRiskPolicy`가 계약과 달리 SAFE를 절대 반환하지 않음

### 위치

`internal/app/risk_policy.go:23-34`

```go
func (DefaultRiskPolicy) Classify(context ResourceContext) RiskAssessment {
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{Level: domain.RiskBlocked, ...}
	}
	return RiskAssessment{Level: domain.RiskReview, ...}  // <- SAFE 분기가 없음
}
```

### 계약과의 불일치

`docs/libra_integration_contracts.md` §20.3 "Risk 중앙 정책"의 MVP 결정표(`CONFIRMED`로 표시됨):

```
| project 내부 산출물이고 재생성 Evidence가 명확함 | SAFE |
```

하지만 실제 코드는 `BLOCKED` 아니면 무조건 `REVIEW`고, `SAFE`로 가는 분기 자체가 없다. `Resource.Regenerable` 필드는 존재하고 실제로 채워지는데(예: node_modules + lockfile 존재 시 `Regenerable=true`), `RiskPolicy`가 이 필드를 아예 안 읽는다.

### 실제 영향

`libra summary`에서 재생성 가능한 `node_modules`(lockfile 있음)도 "Safely reclaimable"이 아니라 "Needs review"에 잡힌다. schedule 문서 §4.1의 SAFE 예시("`node_modules`, `bin`, `obj`, `dist`")가 실제로는 한 번도 SAFE로 안 나온다는 뜻 — Day 5 `plan`/`clean` 구현의 전제 조건이 비어있는 상태.

*(참고: 이 문제는 지난 세션에서 이미 한 번 발견해서 PR #19/#20 설명에 기록해뒀었다. 아직 안 고쳐져 있어서 이번에 다시 확인.)*

### 제안

`internal/app`는 공동 소유이고 `RiskPolicy`는 A가 만든 영역(PR #5)이니, A가 §19.3(산출물 판정 최소 조건: project root 내부 / 알려진 output path / reparse point 아님 / Git tracked 원본 없음 / 재생성 Evidence 존재)을 참고해서 SAFE 분기를 추가하는 게 맞다. Mac C가 대신 고치지 않고 여기 기록만 남긴다 — `docs/libra_collaboration_rules.md`의 "다른 영역 버그는 직접 고치지 않고 이슈로 남긴다" 원칙에 따름. **필요하면 이 항목으로 별도 issue를 파는 걸 제안.**

---

## 5. `cmd` 계층이 명령마다 다른 구조를 씀

### 위치와 증거

`docs/libra_collaboration_rules.md` §7 "계층 방향": `cmd → application service → domain → adapter/repository`.

| 명령 | application service 사용 | repository 직접 호출 |
|---|---|---|
| `cmd/scan.go` | `app.NewAnalysisOrchestrator`, `app.NewResourceService` | (orchestrator 내부에서만) |
| `cmd/summary.go` | `app.NewSummaryService` | 없음 |
| `cmd/projects.go` | 없음 | `sqlite.NewProjectRepository`, `sqlite.NewDependencyRepository` 직접 |
| `cmd/resources.go` | 없음 | `sqlite.NewResourceRepository`, `sqlite.NewDependencyRepository` 직접 |
| `cmd/explain.go` | `app.NewExplainService` (일부) | `sqlite.New*Repository` 직접 (대상 식별용) |
| `cmd/impact.go` | `app.NewImpactService` (일부) | `sqlite.New*Repository` 직접 (연결 대상 조회용) |

**참고: `cmd/projects.go`와 `cmd/resources.go`는 둘 다 Mac C(저)가 작성했다** — 이건 다른 사람 탓하는 항목이 아니라 제 코드에도 있는 일관성 문제라 그대로 적었다.

### 왜 문제로 볼 수 있는가 (그리고 왜 아닐 수도 있는가)

`scan`/`summary`는 "여러 단계 로직 + 판정"이 있어서 서비스로 뽑을 이유가 명확하다. `projects`/`resources`는 "목록 조회 + 필터 + 개수 세기"뿐이라 서비스 계층이 그냥 얇은 pass-through가 될 수 있다 — 그래서 지금까지는 실용적으로 생략해왔다. 다만 이게 "의도된 예외"인지 "그냥 빠뜨린 것"인지 문서에 없어서, 다음 사람이 `cmd/*.go`를 새로 만들 때 어느 쪽을 따라야 할지 알 수 없다.

### 제안

둘 중 하나로 팀이 합의해서 문서화하는 걸 제안:
1. "단순 목록+필터는 cmd에서 repository 직접 호출 허용" 을 §7에 명시적 예외로 추가, 또는
2. `ProjectsService`/`ResourcesService`처럼 얇은 서비스를 만들어서 일관성을 맞춤 (당장 급한 리팩터링은 아님).

---

## 6. `DependencyAnalyzer`가 scan에 연결되지 않음 (참고용 재정리)

이미 [issue #22](https://github.com/madcamp-official/26s-w3-c2-01/issues/22)로 등록했고 `docs/libra_integration_contracts.md` §29에도 기록해뒀다. 여기서는 "코드 구조상 이슈" 관점에서만 한 줄로 다시 짚는다:

- 위치: `cmd/scan.go`의 `WithDetectors(..., resourceDetectors(), nil)` — 세 번째 인자가 항상 `nil`.
- `internal/app/project_detector_adapters.go`의 `MSBuildProjectDetector.Observe`가 `parsed[i].Declared`를 버림.
- 담당: Windows B (`internal/adapter/msbuild` + 공동 소유 `internal/app` 계약 변경 필요).

---

## 7. `ScanService`가 죽은 코드로 보임 (2026-07-20 파일별 주석 작업 중 발견)

### 위치

`internal/app/scan_service.go` — `ScanRecord`, `ScanRepository`, `ScanService`, `ScanService.Run`.

### 증거

```
$ grep -rn "ScanService\b" --include="*.go" .
internal/app/scan_service.go:51:type ScanService struct {
internal/app/scan_service.go:57:func NewScanService(...)
internal/app/scan_service.go:64:func (s *ScanService) Run(...)
internal/app/scan_service_test.go:...       (자기 자신 테스트만)
internal/app/scan_service_integration_test.go:...  (자기 자신 테스트만)
```

`cmd/scan.go`는 `NewScanService`가 아니라 `app.NewAnalysisOrchestrator`를 생성해서 쓴다. `ScanService.Run`을 실제로 호출하는 프로덕션 코드가 레포 전체에 하나도 없다 — 자기 자신의 테스트 파일 2개만 이 타입을 쓴다.

다만 `ScanRecord`/`ScanRepository`/`ScanStatus*` 상수는 죽은 게 아니다 — `AnalysisOrchestrator`가 그대로 재사용한다(`analysis_orchestrator.go`의 `scans ScanRepository` 필드).

### 추정 원인

Day2에 `ScanService`로 스캔 파이프라인을 처음 만들었다가(PR #2), 이후 `AnalysisOrchestrator`(PR #12, 프로젝트/리소스/의존성 탐지를 한 파이프라인으로 통합)로 옮겨가면서 `ScanService.Run`은 안 지우고 남겨둔 것으로 보인다. `ScanRecord`/`ScanRepository`는 새 코드도 계속 써서 자연스럽게 안 지워진 듯하다.

### 제안

`ScanService`/`ScanService.Run`과 그 전용 테스트 2개 파일만 삭제하는 걸 제안한다 (`ScanRecord`/`ScanRepository`/`ScanStatus*`는 유지). 다만 이건 A가 만든 영역이니 A 확인 후 진행하는 게 맞다고 봐서 여기 기록만 하고 직접 지우지 않았다.

---

## 8. `cmd/projects.go`의 `--type` 필터만 대소문자를 구분함 (2026-07-20, `cmd/target.go` 설계 근거를 설명하다가 발견)

### 위치

`cmd/projects.go`의 세 필터:

```go
if projectsType != "" && string(project.Type) != projectsType { continue }                          // 대소문자 구분
if projectsDrive != "" && !strings.EqualFold(project.Drive, projectsDrive) { continue }               // 대소문자 무시
if projectsStatus != "" && !strings.EqualFold(string(project.Status), projectsStatus) { continue }    // 대소문자 무시
```

### 왜 문제인가

`--drive`/`--status`는 대소문자를 무시하고 `--type`만 정확히 일치해야 한다 — `libra projects --type Node`는 아무것도 안 찾고 `--type node`만 찾는다. 사용자 입장에서 세 옵션이 왜 다르게 동작하는지 알 방법이 없다. `cmd/resources.go`/`cmd/summary.go`의 같은 종류 필터는 전부 `strings.EqualFold`를 쓴다 — `projects.go`의 `--type`만 예외.

### 제안

`string(project.Type) != projectsType`을 `!strings.EqualFold(string(project.Type), projectsType)`으로 바꾸면 된다. `cmd/projects.go`는 제 소유 영역(Mac C)이라 원하시면 바로 고치겠습니다 — 지금은 기록만 해둡니다.

---

## 확인하지 못한 것 (git/GitHub만으로는 알 수 없음)

- DB schema 변경(§9) 전에 "세 명이 모델 이름과 의미 합의"가 실제로 채팅 등에서 있었는지 — git 기록만으로는 확인 불가. 팀이 직접 확인 필요.
- 사용자님(A/B) 각자 실제 Windows 장비에서 검증했는지(§Day3 B "실제 Windows 환경 검증") — 코드/PR에 결과가 기록되어 있지 않아 확인 불가.
