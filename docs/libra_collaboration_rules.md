# Libra 협업 규칙

> 이 문서는 Libra 프로젝트를 3명이 빠르게 병렬 개발하면서도 `main` 브랜치의 안정성, Windows/macOS 호환성, 파일 정리 기능의 안전성을 지키기 위한 팀 규칙을 정의한다.

---

## 1. 기본 원칙

1. `main` 브랜치는 항상 빌드 및 실행 가능한 상태를 유지한다.
2. 한 사람이 오래 독점하는 큰 기능보다 하루 안에 병합 가능한 작은 작업으로 나눈다.
3. 플랫폼 전용 코드는 공통 로직과 분리한다.
4. 파일 삭제·이동·복구 기능은 일반 기능보다 엄격하게 리뷰한다.
5. 분석 결과가 확실하지 않으면 안전하다고 단정하지 않는다.
6. AI가 생성한 코드는 사람이 이해하고 검증한 뒤 병합한다.
7. 명령어, 데이터 모델, 문서와 실제 구현이 어긋나지 않도록 함께 수정한다.

---

## 2. 역할 및 코드 소유 영역

| 담당 | 주 환경 | 주 역할 | 주요 소유 영역 |
|---|---|---|---|
| Windows A | Windows | 플랫폼·스캔·DB·안전 처리 | `internal/scanner`, `internal/store`, `internal/config`, `internal/safety` |
| Windows B | Windows | Windows SDK·MSBuild·.NET 의존성 | `internal/adapter/windowsdk`, `msbuild`, `visualstudio`, `dotnet` |
| Mac C | macOS | CLI·출력·Node 분석·QA·문서 | `cmd`, `internal/output`, `internal/adapter/node`, `testdata`, `docs` |

### 코드 소유권 원칙

- 코드 소유권은 다른 팀원이 수정하지 못한다는 뜻이 아니다.
- 소유 영역을 수정하는 PR은 해당 담당자를 리뷰어로 지정한다.
- DB schema, domain model, CLI 명령 계약은 공동 소유로 본다.
- cleanup 및 restore 관련 코드는 반드시 Windows A를 포함해 2명 이상이 검토한다.
- Windows 전용 기능도 parser와 domain 로직은 가능한 한 Mac에서 테스트할 수 있게 분리한다.

---

## 3. 브랜치 운영

### 기본 브랜치

```text
main
```

별도의 장기 `develop` 브랜치는 두지 않는다.

### 브랜치 이름

```text
feature/<기능명>
fix/<버그명>
refactor/<대상>
test/<대상>
docs/<문서명>
chore/<작업명>
```

예시:

```text
feature/project-scanner
feature/windows-sdk-adapter
feature/impact-command
fix/windows-path-normalization
test/node-project-fixtures
docs/collaboration-rules
```

### 브랜치 규칙

- 브랜치는 가능한 한 하루 안에 병합한다.
- 하나의 브랜치에서는 하나의 주제만 다룬다.
- 다른 기능까지 함께 고치지 않는다.
- 오래된 브랜치는 매일 `main`을 반영한다.
- 병합이 끝난 브랜치는 삭제한다.
- `main`에 직접 push하지 않는다.
- 긴급 수정도 PR을 거친다.

### 작업 시작 및 동기화

```bash
git switch main
git pull origin main
git switch -c feature/<name>
```

작업 중 `main` 변경 반영:

```bash
git fetch origin
git rebase origin/main
```

충돌이 복잡하거나 이미 공유된 브랜치라면 무리하게 force push하지 말고 팀에 알린 뒤 처리한다.

---

## 4. 커밋 규칙

### 커밋 메시지 형식

```text
<type>(<scope>): <내용>
```

### 권장 type

| Type | 용도 |
|---|---|
| `feat` | 기능 추가 |
| `fix` | 버그 수정 |
| `refactor` | 동작 변화 없는 구조 개선 |
| `test` | 테스트 추가 및 수정 |
| `docs` | 문서 변경 |
| `chore` | 설정, 의존성, CI 등 |
| `perf` | 성능 개선 |

예시:

```text
feat(scanner): add recursive directory scan
feat(msbuild): parse Windows SDK version
fix(path): reject unsafe reparse point targets
test(node): add package project fixtures
docs: add cleanup safety policy
chore(ci): test on Windows and macOS
```

### 커밋 단위

- 빌드 가능한 단위로 커밋한다.
- 기능 추가와 대규모 포맷 변경을 한 커밋에 섞지 않는다.
- 단순 정렬과 실제 로직 변경을 분리한다.
- 비밀키, 개인 경로, 실제 사용자 데이터, 로컬 DB 파일을 커밋하지 않는다.
- 자동 생성 파일은 필요한 경우에만 포함한다.

---

## 5. Pull Request 규칙

### PR 크기

- 한 PR은 하나의 기능 또는 수정만 포함한다.
- 리뷰 가능한 범위로 유지한다.
- 대규모 변경은 인터페이스 PR과 구현 PR로 분리한다.
- 하루 이상 걸리는 기능은 실행 가능한 중간 단계로 나눈다.

### PR 제목

커밋 메시지와 같은 형식을 사용한다.

```text
feat(impact): show projects affected by SDK removal
```

### PR 본문 필수 내용

```markdown
## 작업 내용
- 무엇을 구현하거나 수정했는지

## 실행 방법
- 실제 확인에 필요한 명령어

## 테스트
- 실행한 테스트
- 사용한 fixture 또는 실제 환경

## 영향 범위
- 변경된 명령어, schema, 플랫폼, 출력

## 확인이 필요한 부분
- 리뷰어가 집중해서 봐야 할 지점
```

### CLI 변경 PR 추가 요구사항

다음을 포함한다.

```text
실행 명령
정상 출력 예시
오류 출력 예시
--json 출력 변화 여부
```

### DB 변경 PR 추가 요구사항

- schema 변경 이유
- migration 방법
- 기존 데이터 호환 여부
- rollback 또는 초기화 방법
- 영향을 받는 query와 service

### PR 완료 체크리스트

```text
[ ] gofmt 적용
[ ] go test ./... 통과
[ ] go vet ./... 통과
[ ] Windows 또는 macOS에서 직접 실행
[ ] 새 기능에 테스트 추가
[ ] 오류 케이스 확인
[ ] CLI 변경 시 사용 예시 추가
[ ] 문서와 구현 내용 일치
[ ] 개인 경로나 민감 정보가 포함되지 않음
```

---

## 6. 코드 리뷰 규칙

### 리뷰 인원

| 변경 종류 | 최소 승인 |
|---|---:|
| 일반 기능 | 1명 |
| DB schema 또는 domain model | 2명 또는 팀 합의 |
| 파일 이동·삭제·복구 | 2명 |
| 시스템 경로 안전 정책 | 2명 |
| Windows 전용 탐지 | Windows 팀원 1명 이상 |
| CLI 출력 및 문서 | Mac C 또는 해당 담당 1명 이상 |

### 리뷰 우선순위

1. 데이터 손실 가능성
2. 경로 및 권한 처리
3. 요구사항과 실제 동작 일치 여부
4. Windows/macOS 빌드 호환성
5. 오류 처리
6. 테스트 충분성
7. 코드 가독성과 구조
8. 성능

### 리뷰 댓글 구분

```text
[BLOCKER] 반드시 수정해야 병합 가능
[QUESTION] 의도나 동작 확인 필요
[SUGGESTION] 선택적으로 개선 가능
[NIT] 사소한 스타일 의견
```

### 리뷰 대응

- 모든 댓글에는 수정, 설명, 보류 중 하나로 답한다.
- 단순히 `resolved`만 누르지 않는다.
- 의견이 다르면 요구사항과 안전 원칙을 기준으로 결정한다.
- cleanup 관련 이견이 해소되지 않으면 더 보수적인 동작을 선택한다.

---

## 7. 코드 구조 및 의존성 규칙

### 계층 방향

```text
cmd
  ↓
application service
  ↓
domain
  ↓
adapter / repository
```

### 기본 규칙

- `cmd`에는 파일 스캔, XML 파싱, DB query 같은 로직을 직접 넣지 않는다.
- Adapter가 파일을 직접 삭제하지 않는다.
- cleanup 여부는 중앙 안전 정책에서 판단한다.
- domain 모델은 OS API에 직접 의존하지 않는다.
- 출력 포맷은 서비스 로직과 분리한다.
- Windows API 호출은 build tag가 붙은 파일에 격리한다.
- 공통 parser는 Windows가 아닌 환경에서도 테스트할 수 있어야 한다.
- 순환 import를 만들지 않는다.
- 전역 상태를 최소화한다.
- 가능한 범위에서 `context.Context` 취소를 전달한다.

---

## 8. Windows와 macOS 호환 규칙

프로젝트는 Windows 우선이지만, Mac 팀원도 공통 기능을 개발할 수 있어야 한다.

### Windows 전용 파일

```go
//go:build windows
```

예시:

```text
detector_windows.go
```

### 비Windows 대체 구현

```go
//go:build !windows
```

예시:

```text
detector_unsupported.go
```

비Windows 환경에서는 빌드 실패 대신 명확한 미지원 오류를 반환한다.

### 플랫폼별로 분리할 항목

```text
실제 Windows 설치 탐지
vswhere 실행
Registry 접근
NTFS 및 Windows 경로 처리
```

### 공통 코드로 유지할 항목

```text
.vcxproj XML 파싱
SDK 버전 문자열 해석
의존성 그래프 생성
위험도 계산
CLI 출력
JSON 직렬화
fixture 테스트
```

### 플랫폼 호환 완료 조건

```bash
go test ./...
go vet ./...
go build ./...
```

Windows 전용 테스트는 build tag를 사용한다.

---

## 9. 데이터베이스 및 모델 변경 규칙

### 공동 합의가 필요한 변경

- 테이블 추가 및 삭제
- column 의미 변경
- enum 값 변경
- dependency relation 변경
- 위험도 또는 evidence 종류 변경
- CLI JSON schema 변경

### 변경 절차

1. Issue 또는 짧은 문서로 변경 목적 작성
2. 영향받는 코드 영역 표시
3. 세 명이 모델 이름과 의미 합의
4. migration과 테스트 작성
5. PR에서 예시 데이터 확인

### 금지 사항

- 개인 브랜치에서 schema를 크게 변경한 뒤 통보
- migration 없이 기존 column 의미 변경
- DB enum 문자열을 코드 곳곳에 하드코딩
- raw SQL이 여러 계층에 흩어지는 구조
- 테스트 DB와 실제 DB schema가 다른 상태

---

## 10. CLI 계약 변경 규칙

명령어 이름, 옵션, 출력 필드가 바뀌면 사용자 계약이 바뀐 것으로 본다.

### 변경 시 함께 수정할 항목

- Cobra command
- help 메시지
- README 사용 예시
- Golden output 테스트
- JSON 출력 테스트
- 관련 문서
- 발표 데모 명령

### 명령어 안정성 원칙

- 같은 의미의 옵션 이름을 명령마다 다르게 만들지 않는다.
- 삭제나 격리처럼 위험한 명령은 명시적인 옵션을 요구한다.
- 기본값은 보수적으로 설정한다.
- 일반 텍스트 출력과 JSON 출력의 의미가 일치해야 한다.
- 오류 메시지는 원인과 다음 행동을 함께 설명한다.

---

## 11. 테스트 규칙

### 새 기능의 최소 테스트

새 기능에는 다음 중 하나 이상이 반드시 포함되어야 한다.

- 단위 테스트
- fixture 기반 통합 테스트
- Golden output 테스트
- Windows 전용 통합 테스트

### fixture 규칙

- 실제 개인 프로젝트를 fixture로 사용하지 않는다.
- 최소 구조만 가진 가상 프로젝트를 만든다.
- 절대 경로에 의존하지 않는다.
- 정상 사례와 오류 사례를 함께 둔다.

예시:

```text
testdata/
├─ msbuild/
│  ├─ exact-sdk-version/
│  ├─ inherited-props/
│  └─ unresolved-property/
├─ node/
│  ├─ basic/
│  ├─ malformed-package-json/
│  └─ missing-lockfile/
└─ filesystem/
   ├─ build-output/
   └─ nested-project/
```

### cleanup 테스트

실제 사용자 경로나 시스템 경로에서 테스트하지 않는다. 임시 디렉터리 또는 fixture에서만 실행한다.

검증 항목:

- dry-run에서는 파일이 변경되지 않음
- 원래 경로와 격리 경로가 기록됨
- 시스템 경로가 거부됨
- reparse point가 거부됨
- 충돌 시 덮어쓰지 않음
- restore가 원래 위치를 복구함
- 일부 실패 시 상태가 정확히 기록됨

---

## 12. 파일 정리 기능 안전 규칙

### 기본 정책

- 기본 모드는 항상 `dry-run`이다.
- 직접 삭제보다 격리 이동을 우선한다.
- 시스템 구성요소는 자동 정리하지 않는다.
- 정리 직전에 경로를 다시 검증한다.
- 스캔 이후 변경된 대상은 자동 처리하지 않는다.
- 사용자 원본일 가능성이 있으면 `BLOCKED`로 분류한다.
- 복구 방법이 불명확하면 `BLOCKED`로 분류한다.

### 자동 처리 가능한 대상

엄격한 조건을 통과한 프로젝트 내부 산출물만 허용한다.

```text
node_modules
bin
obj
build
dist
.next
out
```

### 자동 처리 금지 대상

```text
C:\Windows
C:\Program Files
C:\Program Files (x86)
Windows SDK
Visual Studio
.NET Runtime
Docker Volume
데이터베이스
Git objects
.env
인증서
비밀키
사용자 문서
알 수 없는 대용량 폴더
```

### cleanup PR 추가 체크리스트

```text
[ ] dry-run 검증
[ ] 대상 경로 재검증
[ ] 시스템 경로 차단
[ ] reparse point 차단
[ ] transaction 기록
[ ] 실패 시 상태 기록
[ ] restore 테스트
[ ] Windows A 포함 2인 리뷰
```

---

## 13. AI 사용 규칙

AI는 개발 속도를 높이기 위한 도구이며 코드 책임은 팀원에게 있다.

### AI에게 맡기기 적합한 작업

- 반복적인 boilerplate
- fixture 초안
- 테스트 케이스 목록
- XML 및 JSON parser 초안
- CLI help 문구
- 문서 정리
- 간단한 formatter
- 코드 설명 및 리뷰 보조

### 반드시 사람이 직접 검토할 작업

- 파일 삭제·이동·복구
- 경로 정규화
- symlink와 junction 처리
- 권한 처리
- DB migration
- 외부 명령 실행
- shell argument escaping
- 위험도 및 신뢰도 규칙
- 시스템 경로 denylist
- 동시성 및 transaction

### 금지 사항

- 이해하지 못한 AI 코드를 그대로 병합
- AI가 생성한 대규모 변경을 한 번에 적용
- cleanup 코드를 테스트 없이 병합
- AI에게 실제 사용자 파일이나 비밀 정보 전달
- AI가 기존 인터페이스를 임의로 바꾸도록 방치
- 실패 원인을 모르는 상태에서 코드 재생성만 반복

### AI 코드 확인 절차

1. 생성된 코드의 입력과 출력을 설명한다.
2. 오류 및 경계 사례를 확인한다.
3. 테스트를 작성한다.
4. 직접 실행한다.
5. 팀원 리뷰를 받는다.
6. 필요하면 작은 단위로 다시 작성한다.

---

## 14. 문서 관리 규칙

### 문서와 코드 동기화

다음 변경은 문서 수정도 포함한다.

- 새 명령어 추가
- 옵션 변경
- 위험도 의미 변경
- 지원 프로젝트 종류 변경
- 지원 SDK 및 Adapter 변경
- 정리 가능 대상 변경
- 설치 방법 변경
- 알려진 제한 변경

### 문서 위치

```text
README.md
docs/
├─ architecture.md
├─ cli-spec.md
├─ collaboration-rules.md
├─ safety-policy.md
└─ known-limitations.md
```

### 문서 작성 원칙

- 구현된 기능과 계획 중인 기능을 구분한다.
- 미완성 기능을 완료된 것처럼 쓰지 않는다.
- Windows 전용 여부를 명시한다.
- 예제 명령은 실제 실행 가능한 형태로 유지한다.
- 위험한 명령에는 경고와 복구 방법을 함께 쓴다.

---

## 15. 의사소통 규칙

### 진행 공유

각 팀원은 하루 중 최소 두 번 다음을 공유한다.

```text
현재 작업
완료한 내용
막힌 부분
다른 팀원에게 필요한 결정
```

### 즉시 공유해야 하는 변경

- DB schema 변경 필요
- CLI 명령 또는 옵션 변경
- domain 모델 의미 변경
- cleanup 정책 변경
- 일정에 영향을 주는 기술적 장애
- Windows/macOS에서 서로 다른 동작 발견
- 데이터 손실 가능성이 있는 버그

### 막힌 경우

- 30~60분 이상 진전이 없으면 공유한다.
- 문제, 시도한 방법, 오류 메시지를 함께 전달한다.
- 단순히 “안 된다”고만 말하지 않는다.
- 다른 팀원의 작업을 막는 문제는 우선 처리한다.

### 결정 기록

중요 결정은 채팅에만 남기지 않고 Issue, PR 또는 문서에 기록한다.

예시:

```text
Windows SDK는 자동 삭제하지 않는다.
프로젝트 산출물은 삭제 대신 격리한다.
main에는 직접 push하지 않는다.
```

---

## 16. 병합 및 통합 규칙

### 병합 전

```bash
git fetch origin
git rebase origin/main
go test ./...
go vet ./...
go build ./...
```

모든 `main` 병합은 해당 변경의 현재 구현 상태, 사용자 계약 및 남은 제한을
반영한 문서 변경을 반드시 동반한다. 문서 변경이 필요 없다고 판단한 경우에도
PR 본문에 확인한 문서와 변경이 불필요한 이유를 기록한다.

### 병합 방식

- 기본적으로 squash merge를 사용한다.
- PR 제목이 최종 커밋 메시지가 된다.
- 의미 있는 단계별 커밋을 보존할 필요가 있을 때만 merge commit을 사용한다.
- 불필요한 중간 커밋은 `main`에 남기지 않는다.

### 병합 후

- `main` CI를 확인한다.
- 관련 Issue를 닫는다.
- 작업 브랜치를 삭제한다.
- 다음 작업 전에 최신 `main`을 받는다.
- CLI 또는 schema 변경이면 다른 팀원에게 알린다.

### 통합 실패 시

`main`이 깨졌다면 새 기능보다 복구가 최우선이다.

- 원인을 만든 PR 담당자가 우선 대응한다.
- 빠른 수정이 어렵다면 revert한다.
- 깨진 상태에서 다른 PR을 연속 병합하지 않는다.
- 원인과 재발 방지 내용을 Issue 또는 PR에 기록한다.

---

## 17. Definition of Done

기능은 코드를 작성한 것만으로 완료되지 않는다.

```text
[ ] 요구사항 충족
[ ] 공통 구조 준수
[ ] 오류 처리 구현
[ ] 테스트 작성
[ ] Windows 또는 macOS에서 실행 확인
[ ] CI 통과
[ ] 필요한 문서 수정
[ ] 리뷰 승인
[ ] main 병합 후 정상 동작 확인
```

cleanup 관련 기능은 다음도 필요하다.

```text
[ ] dry-run 확인
[ ] 시스템 경로 차단 확인
[ ] 격리 transaction 확인
[ ] restore 확인
[ ] 2인 이상 승인
```

---

## 18. 일정이 촉박할 때의 원칙

- 테스트와 안전 검증을 빼고 기능 수를 늘리지 않는다.
- 미완성 기능을 `main` 기본 흐름에 노출하지 않는다.
- 실험 기능은 flag 또는 별도 브랜치로 둔다.
- 데몬보다 `scan`, `explain`, `impact`, `plan`의 정확성을 우선한다.
- 시스템 구성요소 삭제보다 분석과 공식 제거 안내에 집중한다.
- 발표 직전 대규모 refactor를 하지 않는다.
- 핵심 기능이 안정되지 않았다면 부가 기능을 중단한다.

---

## 19. 최종 체크 요약

### 작업 시작 전

```text
[ ] 최신 main을 받았는가?
[ ] Issue와 완료 조건이 명확한가?
[ ] 다른 팀원 영역과 충돌하지 않는가?
```

### PR 생성 전

```text
[ ] gofmt
[ ] go test ./...
[ ] go vet ./...
[ ] go build ./...
[ ] 테스트와 문서 추가
[ ] 개인 정보와 절대 경로 제거
```

### 병합 전

```text
[ ] 필요한 리뷰 수 충족
[ ] 리뷰 댓글 처리
[ ] CI 통과
[ ] main과 충돌 없음
```

### 위험 기능 병합 전

```text
[ ] dry-run
[ ] 경로 재검증
[ ] 시스템 경로 차단
[ ] transaction
[ ] restore
[ ] 2인 리뷰
```

---

## 관련 통합 계약

A·B·C 사이의 프로젝트 의미, 경로 identity, scanner 전달 값, DB 저장, 오류, 진행률, CLI·JSON 계약은 [`libra_integration_contracts.md`](./libra_integration_contracts.md)를 기준으로 검토한다.

해당 문서에서 `DECISION_REQUIRED`로 표시된 항목은 공용 domain, DB schema 또는 CLI 계약에 영향을 주므로 팀 합의와 필요한 리뷰 없이 구현하지 않는다.
