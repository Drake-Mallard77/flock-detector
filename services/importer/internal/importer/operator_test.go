package importer

import "testing"

func TestClassifyOperator(t *testing.T) {
	cases := map[string]OperatorType{
		"Chicago Police Department":             OperatorLawEnforcement,
		"Volusia County Sheriff's Office":       OperatorLawEnforcement,
		"Warrick county sherriff":               OperatorLawEnforcement, // misspelled in the tags
		"California Highway Patrol (CHP)":       OperatorLawEnforcement,
		"United States Border Patrol":           OperatorLawEnforcement,
		"Sunnyvale Department of Public Safety": OperatorLawEnforcement,
		// A university police force is law enforcement, not education —
		// classifying it as a school would understate what it is.
		"UC Merced Police Department": OperatorLawEnforcement,

		"University of Central Florida": OperatorEducation,

		"City of Beverly Hills":            OperatorGovernment,
		"Town of Bay Harbor Islands":       OperatorGovernment,
		"Transportation Corridor Agencies": OperatorGovernment,

		"Tanger Inc.": OperatorPrivate,

		// Well-known brands are classified from a curated list (see
		// TestClassifyOperator_KnownBrands); unfamiliar names still fall
		// through to unknown rather than being mislabelled.
		"Walmart":         OperatorPrivate,
		"Riverbend Plaza": OperatorUnknown,
		"":                OperatorUnknown,
	}

	for in, want := range cases {
		if got := ClassifyOperator(in); got != want {
			t.Errorf("ClassifyOperator(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalizeOperator(t *testing.T) {
	// Both spellings appear in the real data and split one agency in two.
	curly := CanonicalizeOperator("Volusia County Sheriff\u2019s Office")
	straight := CanonicalizeOperator("Volusia County Sheriff's Office")
	if curly != straight {
		t.Errorf("apostrophe variants did not converge: %q vs %q", curly, straight)
	}

	if got := CanonicalizeOperator("Tucson_Police_Department"); got != "Tucson Police Department" {
		t.Errorf("underscores not normalized: %q", got)
	}
	if got := CanonicalizeOperator("  Chicago   Police  Department "); got != "Chicago Police Department" {
		t.Errorf("whitespace not collapsed: %q", got)
	}
}

// Brands that recur across many states. Naming them is a fact, not a
// guess, and leaving them unclassified made the review queue harder to work
// through than it needed to be.
func TestClassifyOperator_KnownBrands(t *testing.T) {
	for _, in := range []string{"Lowe's", "The Home Depot", "Walmart", "Kaiser Permanente", "UPS"} {
		if got := ClassifyOperator(in); got != OperatorPrivate {
			t.Errorf("ClassifyOperator(%q) = %q, want private", in, got)
		}
	}

	// A brand name inside a public body's name must not flip it: this is a
	// police department, whatever it patrols.
	if got := ClassifyOperator("Target Police Department"); got != OperatorLawEnforcement {
		t.Errorf("law enforcement should win over a brand match, got %q", got)
	}

	// Still no guessing for genuinely unfamiliar names.
	if got := ClassifyOperator("Riverbend Plaza"); got != OperatorUnknown {
		t.Errorf("expected unknown for an unlisted name, got %q", got)
	}
}
