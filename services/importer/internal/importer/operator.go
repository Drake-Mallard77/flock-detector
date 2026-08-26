package importer

import "strings"

// OperatorType is a coarse classification of who runs a camera.
//
// It exists so the atlas can say plainly what kind of claim a record makes.
// "Chicago Police Department" and "The Home Depot" are both real ALPR
// deployments, but presenting them identically in a public-records atlas
// blurs what the site is asserting.
type OperatorType string

const (
	OperatorLawEnforcement OperatorType = "law_enforcement"
	OperatorGovernment     OperatorType = "government"
	OperatorEducation      OperatorType = "education"
	OperatorPrivate        OperatorType = "private"
	OperatorUnknown        OperatorType = "unknown"
)

// Checked in order; the first match wins. Law enforcement is deliberately
// first: "UC Merced Police Department" is a police force that happens to
// sit at a university, and classifying it as education would understate it.
var operatorPatterns = []struct {
	kind     OperatorType
	keywords []string
}{
	{OperatorLawEnforcement, []string{
		"police", "sheriff", "sherriff", // sherriff: a common misspelling in the tags
		"constable", "marshal", "state patrol", "highway patrol",
		"public safety", "law enforcement", "border patrol",
		"customs and border", "security forces", "district attorney",
		"prosecutor", "corrections", "task force",
	}},
	{OperatorEducation, []string{
		"university", "college", "school district", "campus", "academy",
	}},
	{OperatorGovernment, []string{
		"city of", "town of", "village of", "county of", "township",
		"borough", "municipal", "department of transportation",
		"transportation corridor", "transit", "port authority",
		"housing authority", "parks", "public works", "state of",
		"dept of", "department of",
	}},
}

// Suffixes that mark a company rather than a public body.
var privateMarkers = []string{
	"inc.", "inc", "llc", "l.l.c.", "corp", "corporation", "company",
	"holdings", "properties", "property management", "management",
	"apartments", "hoa", "homeowners", "casino", "mall", "market",
	"markets", "stores", "store",
}

// ClassifyOperator guesses what kind of entity an operator name describes.
//
// Keyword matching, so it will be wrong sometimes — which is why classified
// candidates still go to a human for review rather than being published.
// Anything it can't place returns "unknown" rather than being guessed into
// a category, so a reviewer sees the uncertainty instead of a confident
// mislabel.
func ClassifyOperator(operator string) OperatorType {
	name := strings.ToLower(strings.TrimSpace(operator))
	if name == "" {
		return OperatorUnknown
	}

	for _, p := range operatorPatterns {
		for _, kw := range p.keywords {
			if strings.Contains(name, kw) {
				return p.kind
			}
		}
	}

	for _, marker := range privateMarkers {
		// Word-boundary-ish check so "incorporated" in a city name doesn't
		// read as the "inc" suffix.
		if strings.HasSuffix(name, " "+marker) || name == marker {
			return OperatorPrivate
		}
	}

	return OperatorUnknown
}

// CanonicalizeOperator makes cosmetically-different spellings of the same
// operator group together.
//
// Real duplicates observed in the data: "Volusia County Sheriff's Office"
// and "Volusia County Sheriff’s Office" (straight vs curly apostrophe), and
// "Tucson_Police_Department" vs "Tucson Police Department". Each split one
// agency into two candidate records.
func CanonicalizeOperator(operator string) string {
	s := strings.TrimSpace(operator)
	// Unicode punctuation that a phone keyboard or copy-paste introduces.
	s = strings.NewReplacer(
		"\u2019", "'", // right single quote
		"\u2018", "'", // left single quote
		"\u201C", `"`,
		"\u201D", `"`,
		"\u2013", "-", // en dash
		"\u2014", "-", // em dash
		"_", " ",
	).Replace(s)
	// Collapse runs of whitespace left by the substitutions.
	return strings.Join(strings.Fields(s), " ")
}
