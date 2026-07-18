package domain

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
