package domain

// Dependency is an edge from a BuildProject to a Resource it relies on,
// backed by one or more pieces of Evidence (e.g. a DECLARED reference from a
// .vcxproj plus an OBSERVED reference from a later scan).
type Dependency struct {
	ID             string
	BuildProjectID string
	ResourceID     string
	Evidence       []Evidence
}
