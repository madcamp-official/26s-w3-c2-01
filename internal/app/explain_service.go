// [파일 역할] 현재 이 파일은 `package app` 한 줄만 있는 빈 스텁이다(2026-07-19
// 기준, main 브랜치 상태). 이름과 dependency_repository.go의 주석("app.ImpactService와
// app.ExplainService 모두 이 인터페이스에만 의존")으로 미루어 보면 impact_service.go의
// ImpactService(영향 판정)와 짝을 이루어 "왜 이 PROJECT -> RESOURCE 의존성이
// 존재한다고 판단했는지"를 dependency_repository.go의 DependencyRepository /
// domain/evidence.go의 Evidence를 이용해 설명하는 서비스가 될 것으로 보이나,
// 실제 구현은 아직 이 브랜치에 병합되지 않은 별도 브랜치/PR(#24)에 있다. 다 구현된
// 것처럼 취급하지 말 것 — 지금은 아무 타입도, 아무 함수도 없다.
package app
