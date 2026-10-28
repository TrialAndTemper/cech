package build

import (
	"fmt"
	"slices"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/jsonedit"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// Manifest returns p's plugin.json with its version set to version
// (ADR 14 item 3), its description copied from SKILL.md and, when
// catalogue is not "", the marketplace of each dependency on a CECH
// plugin set to it (ADR 20 item 3). Only those fields change; every
// other byte stays as written (ADR 18 item 6). A new field goes right
// after name, so the description comes before the version.
func Manifest(p marketplace.Plugin, version, catalogue string) ([]byte, error) {
	out, err := jsonedit.SetString(p.Manifest, "version", version, "name")
	if err == nil {
		out, err = jsonedit.SetString(out, "description", p.Description, "name")
	}
	if err == nil && catalogue != "" {
		out, err = setCECHMarketplace(out, catalogue)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.ManifestPath(), err)
	}
	return out, nil
}

// setCECHMarketplace sets the marketplace of each dependency in src
// written as an object whose name starts with cech-. It leaves other
// dependencies, such as a plain string, to the validator.
func setCECHMarketplace(src []byte, catalogue string) ([]byte, error) {
	spans, _, err := jsonedit.Elements(src, "dependencies")
	if err != nil {
		return nil, err
	}
	for _, s := range slices.Backward(spans) {
		dep := src[s.Start:s.End]
		if dep[0] != '{' {
			continue
		}
		name, _, err := jsonedit.String(dep, "name")
		if err != nil {
			return nil, fmt.Errorf("dependencies: %w", err)
		}
		if !strings.HasPrefix(name, "cech-") {
			continue
		}
		edited, err := jsonedit.SetString(dep, "marketplace", catalogue, "name")
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", name, err)
		}
		src = slices.Concat(src[:s.Start], edited, src[s.End:])
	}
	return src, nil
}
