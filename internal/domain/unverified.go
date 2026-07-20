package domain

// [파일 역할] domain.UnverifiedScope는 의존성 분석 중 "확인은 했지만 결과를
// 판단할 수 없었던 범위"를 "평가했는데 실제로 없음(evaluated-and-empty)"과
// 구분하기 위한 모델이다. 예를 들어 Configuration/Platform Condition으로
// 게이트된 MSBuild PropertyGroup처럼, 선언 자체는 실재하지만 현재 빌드
// 구성에 적용되는지는 알 수 없는 경우를 표현한다. internal/adapter/msbuild
// /resolve.go의 dependency-resolution 로직이 이 타입 값을 직접 채운다.
// 주의: internal/app/analysis_contract.go에도 이름이 같은 app.UnverifiedScope
// (Path/Phase/Reason 필드만 있는 더 단순한 구조체)가 별도로 있다 — 그쪽은
// DetectionResult[T].Unverified가 쓰는 "파일/디렉터리 단위" 미검증 범위이고,
// 이 domain.UnverifiedScope는 BuildProjectID/Source/Property/Condition까지
// 담는 "의존성 그래프 단위" 미검증 범위로, 서로 다른 목적의 별개 타입이다.

// UnverifiedScope records a part of dependency analysis that was not
// evaluated, as distinct from evaluated-and-empty (see
// docs/libra_integration_contracts.md §19.1). For example, an MSBuild
// PropertyGroup gated by a Configuration/Platform Condition: the
// declaration is real, but whether it applies to the build configuration in
// use is unknown, not absent.
type UnverifiedScope struct {
	BuildProjectID string
	Source         string
	Property       string
	RawValue       string
	Condition      string
	Reason         string
}
