# Libra Python·Conda scope 확장 결정 기록

> 작성일: 2026-07-21
>
> 이 문서는 `docs/libra_integration_contracts.md` §19(분석기별 경계) 스타일로
> Python/conda adapter를 추가하기 전 `DECISION_REQUIRED` 항목을 하나씩
> 합의해가는 임시 작업 문서다. 모든 항목이 `CONFIRMED`되면 이 문서의 내용을
> `libra_integration_contracts.md`에 §19.4(Python)/§19.5(Conda)로 흡수하고,
> 도메인·설정·문서 변경을 한 번에 구현한다 (`libra_collaboration_rules.md` §9
> "공동 합의가 필요한 변경" 절차를 따름).

## 상태 표기

`DECISION_REQUIRED` → 논의 중 · `CONFIRMED` → 합의 완료, 구현 대기 · `IMPLEMENTED` → 코드 반영 완료

> 갱신(2026-07-21): 전 항목 `IMPLEMENTED`. 계약 요약은
> `libra_integration_contracts.md` §19.4(Python)/§19.5(Conda)로 흡수했다.
> 이 문서는 각 결정의 상세 근거(왜 이렇게 정했는지) 기록으로 계속 남긴다.

## 요약 인덱스 (전항목 CONFIRMED, 2026-07-21)

| # | 항목 | 결론 |
|---|---|---|
| 4 | conda 환경 소유권 모델 | REQUIRES-only 공유 리소스 (로컬 prefix env는 예외적으로 OWNS — 결정 5) |
| 6 | 자동 cleanup 대상 범위 | 캐시(`__pycache__` 등)는 항상 allowlist, `.venv`는 PINNED 이상만 |
| 2 | 재생성 가능성 근거 | DECLARED/`PINNED`(신설)/INFERRED/UNKNOWN 4단계 |
| 3 | venv 탐지 기준 | 이름 + `pyvenv.cfg` 존재 확인 필수 |
| 5 | project↔conda env 연결 | `environment.yml` name 매칭(DECLARED), 일반적 이름은 REVIEW, 로컬 prefix는 OWNS 예외 |
| 1 | 프로젝트 마커 우선순위 | `pyproject.toml` > `Pipfile` > `setup.py` > `requirements.txt`(+.py 파일 존재 필요), PrimaryMarker/SecondaryMarkers 구조 |
| 9 | 전역 캐시 범위 | 이번 scope 제외, `PLANNED` |
| 7 | conda 미설치 처리 | `dotnet.CLISDKLister`와 동일: 빈 결과, 에러 아님, 플랫폼 무관 |
| 8 | 담당자 배정 | 이 브랜치(`feature/python_scope`) 작업자가 `internal/adapter/python`·`internal/adapter/conda` 신규 소유 |

이 인덱스 아래 각 항목의 상세 근거는 원래 논의 순서(4→6→2→3→5→1→9→7→8)대로 남긴다.

---

## 결정 4. conda 환경의 소유권 모델 (`CONFIRMED`, 2026-07-21)

**결론: REQUIRES-only, 공유 리소스로 취급한다.**

`windowsdk`/`dotnet-sdk`와 동일한 패턴을 따른다. conda 환경은 프로젝트 폴더
내부에 있든(`-p ./envs`) 전역 named 환경이든 관계없이 project → resource
그래프에서 항상 `RelationRequires` edge만 갖고, `RelationOwns`는 갖지 않는다.

- `CleanupEligible`의 `ProjectOwned` 조건이 애초에 성립하지 않으므로 conda
  환경은 §7의 SAFE 판정 경로에 들어가지 않는다 — 별도의 예외 로직이나
  "다중 참조 감지" 로직을 만들 필요가 없다.
- `libra_collaboration_rules.md` §12 "자동 처리 금지 대상"에 conda 환경을
  Windows SDK/Visual Studio와 같은 급으로 추가한다.
- scan은 conda 환경을 **정보 제공**(어떤 프로젝트가 이 환경을 참조하는지,
  크기가 얼마인지) 목적으로만 다루고, `plan`/`clean`의 자동 선택 후보에는
  올리지 않는다. 수동 삭제 안내(공식 `conda env remove` 명령 제시)는 추후
  `explain`/`impact` 출력에서 다룰 수 있다(§20 "확정 전 impact 문구" 참고,
  범위는 이번 결정에 포함하지 않음).
- 결정 5(project ↔ conda env 연결 근거)가 이 REQUIRES edge를 만드는
  구체적인 매칭 규칙을 정한다.
- **보강(결정 5, 2026-07-21)**: 프로젝트 폴더 내부의 로컬 prefix 환경
  (`conda create -p ./envs`)은 이 REQUIRES-only 원칙의 예외로, venv와
  동일하게 `RelationOwns`로 취급한다 — 자세한 내용은 결정 5 참고.

---

## 결정 6. 자동 cleanup 대상 포함 여부 (`CONFIRMED`, 2026-07-21)

**결론: 순수 캐시는 항상 allowlist, `.venv`/`venv`는 결정 2의 `PINNED` 이상일 때만.**

- `__pycache__`, `.pytest_cache`, `.mypy_cache`, `*.egg-info`는 컴파일/테스트
  캐시일 뿐이라 항상 allowlist 후보(§8 basename allowlist에 추가). 마커
  존재 여부와 무관하게 재생성 가능 — node_modules보다도 안전한 케이스.
- `.venv`/`venv`는 소유 프로젝트의 lockfile 등급(결정 2)이 `PINNED` 이상
  (`PINNED` 또는 `DECLARED`)일 때만 allowlist에 올린다. `INFERRED`/`UNKNOWN`
  등급이면 항상 `REVIEW`로 남기고 자동 선택하지 않는다 — venv 재설치는
  node_modules 재설치보다 시간이 오래 걸리고, 버전 미고정 상태에서
  재현성을 시스템이 보장할 수 없기 때문.
- 이 규칙은 `internal/app/project_detector_adapters.go`의
  `projectArtifactCleanupEvidence`에 대응하는 Python 버전에서, `Regenerable`
  필드를 캐시는 무조건 true, venv는 lockfile 등급 조건부로 설정하는 방식으로
  구현한다(Node의 `hasBuildScript` 게이팅과 같은 자리).

---

## 결정 2. 재생성 가능성(lockfile) 근거 기준 (`CONFIRMED`, 2026-07-21)

**결론: Node의 DECLARED/INFERRED 이분법을 그대로 쓰지 않고 3~4단계로
세분화한다.** 이유: `requirements.txt`는 `package-lock.json`과 달리 버전이
안 고정된 경우가 흔해서, 이분법을 그대로 쓰면 "재생성 가능"이라는 주장이
실제로는 `pip install -r requirements.txt`가 나중에 다른 버전을 설치할
위험을 은폐하는 거짓 확신(false confidence)이 된다.

등급(강한 근거 → 약한 근거):

| 조건 | Evidence 등급 |
|---|---|
| `poetry.lock` / `Pipfile.lock` / `uv.lock` 존재 | `DECLARED` (Node의 lockfile 케이스와 동일) |
| `requirements.txt`만 있고 전체 항목이 `==`로 버전 고정 | 새 중간 등급 `PINNED` |
| `requirements.txt`는 있지만 버전 핀이 부분적이거나 없음(`flask`, `flask>=2.0`) | `INFERRED` |
| 마커 없이 `.py` 파일만 존재 | `INFERRED`도 아닌 별도 `UNKNOWN` — 재생성 가능 여부를 시스템이 주장하지 않음 |

`PINNED`는 `domain.EvidenceKind`에 새로 추가해야 하는 도메인 확장이므로
(§20.2 "확정 전에는 스케일을 임의로 만들지 않는다"의 예외), Confidence 값도
함께 정해야 한다 — DECLARED(75)와 INFERRED(40) 사이의 값(예: 60)을
`domain.DefaultConfidence`에 추가하는 안을 다음 구현 단계에서 팀 검토.
자동 cleanup 후보(결정 6)에 올릴 최소 등급은 `PINNED` 이상으로 제한한다.

---

## 결정 3. venv 탐지 신뢰도 기준 (`CONFIRMED`, 2026-07-21)

**결론: `pyvenv.cfg` 존재를 필수 확인 조건으로 둔다.**

디렉터리 이름(`.venv`/`venv`/`env`)만으로는 판단하지 않는다. 후보 이름과
일치하는 디렉터리를 발견해도, 그 내부에 `pyvenv.cfg`가 실제로 있어야
`ResourceTypeVenv`로 확정한다. Node의 `package.json` 존재 확인과 같은 급의
"내용 기반 확인"이며, 특히 `env`라는 이름은 프로젝트 설정용 일반 폴더와
충돌 가능성이 높아 이름만으로는 오탐 위험이 크다는 점이 결정 근거.

- 후보 디렉터리명: `.venv`, `venv`, `env`, `.env`(디렉터리인 경우만 —
  `.env` 파일은 별개로 denylist §8 대상)
- 확인 실패(이름은 맞지만 `pyvenv.cfg` 없음) 시 `ResourceTypeVenv`가 아닌
  일반 미분류 디렉터리로 남기고 project artifact 후보에서 제외한다 —
  Node의 malformed manifest 케이스처럼 조용히 삭제 후보로 만들지 않는다.
- `pyvenv.cfg`를 열어 `home = ...` 값을 파싱하면 어떤 Python 인터프리터로
  생성됐는지까지 알 수 있으나, 이번 결정 범위는 "venv 여부 확인"까지만이고
  interpreter 버전 파싱은 후속 과제로 남긴다.

---

## 결정 5. project ↔ conda env 연결 근거 (`CONFIRMED`, 2026-07-21)

**결론: `environment.yml`의 `name` 필드 매칭을 1순위로, 로컬 prefix 환경은
결정 4의 예외로 별도 처리한다.**

| 케이스 | 판정 |
|---|---|
| `environment.yml`/`environment.yaml`의 `name:` 필드 ↔ `conda env list`가 반환한 실제 전역 환경 이름이 일치 | `DECLARED` 등급 `RelationRequires` edge 생성 |
| `environment.yml`이 없거나, 이름은 있지만 `base`/`env`/`py39`처럼 프로젝트 고유성이 없는 일반적인 이름 | `REVIEW`로 강등, edge는 만들되 `UnverifiedScope` 기록 — 오탐 시 다른 프로젝트가 쓰는 공유 환경을 잘못 태깅할 위험이 커서, 애매하면 자동 판정보다 사람 확인 쪽을 택함 |
| 로컬 prefix 환경(`conda create -p ./envs`, 프로젝트 폴더 내부) | **결정 4 원칙의 예외.** 위치 자체가 프로젝트 소유의 증거이므로 이름 매칭이 아니라 경로 포함 관계로 판정하고, venv와 동일하게 `RelationOwns`로 취급한다. 전역 named 환경(이름으로만 느슨하게 참조)과는 리스크 구조가 다르다는 점을 코드 주석과 §19.5 문서에 명확히 구분해 남긴다. |

이 예외로 결정 4를 다음과 같이 보강한다: **"conda 환경은 기본적으로
REQUIRES-only 공유 리소스로 취급하되, 프로젝트 폴더 내부의 로컬 prefix
환경만은 venv와 같은 OWNS 리소스로 취급한다."** (판정 기준은 결정 3의
venv와 동일하게 경로 포함 + 내용 확인 — conda prefix 환경도 내부에
`conda-meta/history` 같은 확인 가능한 흔적이 있어 이름만으로 오탐하지
않도록 후속 구현에서 확정한다.)

---

## 결정 1. 프로젝트 마커 우선순위 (`CONFIRMED`, 2026-07-21)

> 답변 원문의 헤드라인 순서(`pyproject.toml > setup.py > Pipfile > requirements.txt`)와
> 본문 근거 설명(Pipfile을 pyproject.toml 바로 다음으로 둠)이 서로
> 달랐다. 본문 근거("Pipfile은 Pipenv 전용 시그널이라 배타적으로 존재하는
> 경우가 많아 pyproject.toml 다음에 두면 충돌이 거의 없다")가 더 명확한
> 이유를 제시하므로 이 순서를 채택했다. **틀렸다면 정정 필요.**

**결론: "우선순위"가 아니라 "동시 존재 허용 + 대표 마커(PrimaryMarker)
지정" 문제로 재정의한다.** Node의 "package.json 단일 마커" 모델과 달리
Python은 여러 마커가 한 디렉터리에 공존하는 게 흔하므로, 하나만 선택하고
나머지를 버리지 않는다.

```text
PrimaryMarker 우선순위: pyproject.toml > Pipfile > setup.py > requirements.txt
```

- **pyproject.toml 존재 시 무조건 PrimaryMarker.** PEP 517/518 이후 사실상
  표준이고 Poetry/Hatch/PDM/uv가 모두 이 파일을 쓴다.
- **Pipfile**은 Pipenv 전용 시그널로 다른 마커와 배타적으로 존재하는 경우가
  많아 2순위로 둬도 충돌이 거의 없다.
- **setup.py 단독**은 레거시 setuptools 프로젝트 — PrimaryMarker로만
  쓰이고, 다른 마커가 이미 매칭됐으면 SecondaryMarkers로만 기록한다.
- **requirements.txt 단독은 가장 약한 시그널.** "진짜 배포 가능한
  프로젝트"인지 "스크립트 모음"인지 애매하므로, Node의 `CanDetect`가
  `package.json`만 확인하는 것과 달리 **requirements.txt 단독인 경우는
  탐지 조건을 완화하지 않고 강화**한다 — 같은 디렉터리에 `.py` 파일이
  최소 1개 이상 있어야 프로젝트로 인정한다.
- 발견된 나머지 마커는 `SecondaryMarkers` 배열로 함께 기록한다 — 당장
  로직에 쓰이진 않지만, 추후 Node의 Origin 필드류 표시 확장이나 결정 2의
  lockfile 탐색(예: pyproject.toml 프로젝트인데 `requirements.txt`가
  보조로 있는 경우) 근거로 재사용 가능.

---

## 결정 9. 전역 캐시(pip/conda) 범위 포함 여부 (`CONFIRMED`, 2026-07-21)

**결론: 이번 scope에서 제외, `PLANNED`로 남긴다.**

`pip cache`(`~/.cache/pip`, Windows `%LOCALAPPDATA%\pip\Cache`)와 conda
package cache(`envs/pkgs`)는 Node의 npm/pnpm 전역 캐시(README §23 Phase 2
"pnpm·npm 전역 캐시 Adapter", 아직 미구현)와 같은 급으로 후속 범위로
미룬다. 이번 Python/conda scope는 다음까지만 다룬다.

```text
포함: Python 프로젝트 탐지, venv/캐시 리소스, conda 환경 REQUIRES/OWNS 연결
제외(PLANNED): pip 전역 캐시, conda 전역 pkgs 캐시
```

이유: 전역 캐시는 여러 사용자·여러 conda root 설치 위치를 탐색해야 하고
(`conda info --root`, `pip cache dir` 등 별도 명령 필요), 결정 4~5로 이미
범위가 넓어진 이번 작업에 추가하면 "일정이 촉박할 때는 기능 수를 늘리지
않는다"(`libra_collaboration_rules.md` §18)는 팀 원칙에 어긋난다.

---

## 결정 7. conda 미설치 환경 처리 (`CONFIRMED`, 2026-07-21)

**결론: `dotnet.CLISDKLister` 패턴을 그대로 따른다 — 빈 결과, 에러 아님.**

- `conda`/`conda.bat` 실행 파일을 PATH에서 찾지 못하면 "설치된 conda 환경
  없음"이라는 유효한 결과로 처리하고 빈 리스트를 반환한다. Issue나 경고를
  남기지 않는다 — pip/venv만 쓰는 사용자가 다수일 것이므로 매 스캔마다
  노이즈가 되는 것을 피한다.
- `adapter.RequireWindows`와 달리 **플랫폼 무관하게 적용**한다. conda는
  macOS/Linux에도 정상 설치되므로 Windows 전용 가드를 씌우지 않는다 —
  대신 실행 파일 탐색은 `os/exec.LookPath("conda")`로 플랫폼 중립적으로
  구현한다(Windows에서는 `conda.bat`/`conda.exe`를 `LookPath`가 알아서
  찾음).
- shell-out 실행 자체(즉 `conda`가 있고 실행했는데 실패하는 경우, 예:
  손상된 설치)는 dotnet과 동일하게 진짜 에러로 취급해 `IssueAdapterFailed`
  로 기록한다 — "conda가 없음"과 "conda가 있는데 실행이 깨짐"은 구분한다.

---

## 결정 8. 담당자 배정 (`CONFIRMED`, 2026-07-21)

**결론: `feature/python_scope` 브랜치 작업자가 `internal/adapter/python`,
`internal/adapter/conda`를 새 독립 소유 영역으로 맡는다.**

기존 3인 표(Windows A/B, Mac C)에 끼워 넣지 않고 별도 행으로 추가한다.
`libra_collaboration_rules.md` §2 표에 다음 행을 추가하는 것을 후속 문서
PR에서 반영한다.

```text
| 담당 | 주 환경 | 주 역할 | 주요 소유 영역 |
|---|---|---|---|
| (branch 작업자) | - | Python·conda 분석 | internal/adapter/python, internal/adapter/conda |
```

- cleanup/restore 관련 코드는 §6 리뷰 규칙대로 Windows A를 포함해 2인 이상
  리뷰가 여전히 필요하다(결정 6에서 venv를 allowlist에 넣기로 했으므로
  해당 코드 경로가 이 조건에 해당).
- conda의 Windows 배치 스크립트 실행(결정 7)에서 실제 문제가 발견되면
  Windows 팀원과 협업한다.
