package importer

import "strings"

// OSM's manufacturer tag is free text, so the same vendor arrives spelled
// several ways ("Verkada" / "Verkada Inc.", "Axon" / "Axon Enterprise",
// "Leonardo" / "Leonardo US Cyber and Security Solutions, LLC"). Left alone
// these filter as separate vendors, which makes the atlas look like it
// can't count.
//
// Mapping is deliberately conservative: it only collapses names that are
// unambiguously the same company. Anything unrecognized passes through
// as-is rather than being guessed at or dropped — a vendor we haven't seen
// before should show up, not disappear.
var manufacturerAliases = map[string]string{
	"verkada":      "Verkada",
	"verkada inc":  "Verkada",
	"verkada inc.": "Verkada",

	"axon":            "Axon",
	"axon enterprise": "Axon",

	"leonardo": "Leonardo",
	"leonardo us cyber and security solutions, llc": "Leonardo",
	"leonardo us cyber and security solutions llc":  "Leonardo",

	"motorola":            "Motorola Solutions",
	"motorola solutions":  "Motorola Solutions",
	"flock":               "Flock Safety",
	"flock safety":        "Flock Safety",
	"genetec":             "Genetec",
	"rekor":               "Rekor",
	"axis":                "Axis Communications",
	"axis communications": "Axis Communications",
	"neology":             "Neology",
	"neology, inc.":       "Neology",
	"neology inc":         "Neology",
}

// Values that mean "we don't know" and should be stored as NULL rather than
// becoming a literal vendor named "Unknown" in the filter list.
var manufacturerUnknown = map[string]bool{
	"unknown":     true,
	"unspecified": true,
	"n/a":         true,
	"none":        true,
	"?":           true,
}

// NormalizeManufacturer maps a raw OSM manufacturer tag to a canonical
// vendor name. Returns nil when the tag is absent or means "unknown", so
// the column stays NULL and the UI can present it as not recorded.
func NormalizeManufacturer(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	key := strings.ToLower(trimmed)
	if manufacturerUnknown[key] {
		return nil
	}

	if canonical, ok := manufacturerAliases[key]; ok {
		return &canonical
	}

	// Unrecognized vendor: keep the original spelling rather than dropping
	// or guessing at it.
	return &trimmed
}
