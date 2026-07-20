package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ImpactView is the rendered result of `libra impact`: every project
// affected by removing a resource, broken down by RUN/BUILD/DEBUG scope,
// plus a recovery hint. See F-08/3.7 in
// docs/libra_cli_commands_and_schedule.md.
type ImpactView struct {
	Projects []ImpactProjectView `json:"projects"`
}

// ImpactProjectView is one affected project's full scope breakdown.
type ImpactProjectView struct {
	ProjectName string            `json:"project_name"`
	ProjectPath string            `json:"project_path"`
	Scopes      []ImpactScopeLine `json:"scopes"`
	Recovery    string            `json:"recovery"`
}

// ImpactScopeLine is one scope's judged impact, plus the phrase an output
// formatter derives from it -- domain values are enums, not sentences, per
// §20.4 of docs/libra_integration_contracts.md.
type ImpactScopeLine struct {
	Scope  domain.ImpactScope `json:"scope"`
	Level  domain.ImpactLevel `json:"level"`
	Phrase string             `json:"phrase"`
}

// RenderText implements Renderable.
func (v ImpactView) RenderText(w io.Writer) error {
	fmt.Fprintf(w, "Affected projects: %d\n", len(v.Projects))
	for _, project := range v.Projects {
		fmt.Fprintln(w)
		fmt.Fprintln(w, project.ProjectPath)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, scope := range project.Scopes {
			fmt.Fprintf(tw, "%s\t%s\n", scope.Scope, scope.Phrase)
		}
		fmt.Fprintf(tw, "RESTORE\t%s\n", project.Recovery)
		tw.Flush()
	}
	return nil
}

// ImpactPhrase turns a domain impact level into the user-facing phrase for
// scope, e.g. "expected to fail". This conversion belongs in the output
// layer, not domain, per §20.4.
func ImpactPhrase(scope domain.ImpactScope, level domain.ImpactLevel) string {
	switch level {
	case domain.ImpactLevelHigh:
		switch scope {
		case domain.ImpactScopeDebug:
			return "expected to fail if build runs"
		case domain.ImpactScopeRun:
			return "may fail to launch"
		default:
			return "expected to fail"
		}
	case domain.ImpactLevelLow:
		return "likely unaffected"
	case domain.ImpactLevelNone:
		return "unaffected"
	default:
		return "unknown"
	}
}

// RecoveryHint gives a short, type-specific recommendation for restoring a
// removed resource. It is deliberately generic MVP copy, not a precise
// per-installation instruction.
func RecoveryHint(resourceType domain.ResourceType) string {
	switch resourceType {
	case domain.ResourceTypeWindowsSDK:
		return "reinstall via the Visual Studio Installer or the standalone Windows SDK installer"
	case domain.ResourceTypeNetFXSDK:
		return "reinstall via the .NET Framework Developer Pack installer"
	case domain.ResourceTypeVisualStudio, domain.ResourceTypeMSBuild:
		return "reinstall or modify via the Visual Studio Installer"
	case domain.ResourceTypeDotNetSDK:
		return "reinstall via the .NET SDK installer"
	case domain.ResourceTypeNodeModules:
		return "reinstall with npm/yarn/pnpm install"
	case domain.ResourceTypeBuildOutput:
		return "regenerate by rebuilding the project"
	case domain.ResourceTypeGlobalCache:
		return "packages are re-downloaded to the cache on next install"
	case domain.ResourceTypeDockerCache:
		return "images and layers are re-pulled or rebuilt on next use"
	default:
		return "no known recovery method"
	}
}
