package comms

import "errors"

// Requirements describe evidence shape, never executable gates or approvals.
type HandoffRequirements struct {
	RequireArtifacts   bool     `json:"require_artifacts,omitempty"`
	RequireChecks      bool     `json:"require_checks,omitempty"`
	RequireLimitations bool     `json:"require_limitations,omitempty"`
	Artifacts          []string `json:"artifacts,omitempty"`
	Checks             []string `json:"checks,omitempty" jsonschema:"Required evidence names, matching check.name"`
}

func validateHandoff(h HandoffRequirements) error {
	for _, v := range append(append([]string{}, h.Artifacts...), h.Checks...) {
		if err := required("handoff requirement", v); err != nil {
			return err
		}
	}
	return nil
}
func validateResultEvidence(h HandoffRequirements, r AssignmentResult) error {
	for _, v := range r.Artifacts {
		if err := required("artifact reference", v); err != nil {
			return err
		}
	}
	for _, v := range r.Limitations {
		if err := required("limitation", v); err != nil {
			return err
		}
	}
	if h.RequireLimitations && r.Limitations == nil {
		return errors.New("handoff requires limitations; supply [] explicitly if none are known")
	}
	// A failure must remain reportable when the intended evidence could not be
	// produced. It creates review attention, not successful gate completion.
	if r.Outcome == "failed" {
		if len(r.Limitations) == 0 {
			return errors.New("failed results must explain limitations or missing evidence")
		}
		return nil
	}
	if h.RequireArtifacts && len(r.Artifacts) == 0 {
		return errors.New("handoff requires artifact references")
	}
	if h.RequireChecks && len(r.Checks) == 0 {
		return errors.New("handoff requires checks performed")
	}
	for _, expected := range h.Artifacts {
		if !oneOf(expected, r.Artifacts...) {
			return errors.New("missing required artifact reference: " + expected)
		}
	}
	for _, expected := range h.Checks {
		found := false
		for _, check := range r.Checks {
			if check.Name == expected {
				found = true
			}
		}
		if !found {
			return errors.New("missing required check evidence: " + expected)
		}
	}
	return nil
}
