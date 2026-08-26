package importer

import "strings"

// OSM's manufacturer tag is free text, so one vendor arrives spelled many
// ways: legal entity names ("Flock Group Inc." is Flock Safety), corporate
// suffixes, product model numbers, case differences, and outright typos.
// Left alone they filter as separate vendors, which makes the atlas look
// like it can't count — and understates the market leader badly. At full US
// scale, Flock alone was split across three spellings.
//
// The mapping is deliberately conservative: it collapses only names that
// are unambiguously the same company. Anything unrecognized passes through
// with its original spelling rather than being guessed at or dropped, so a
// genuinely new manufacturer shows up instead of vanishing.
//
// Keys are lowercased; NormalizeManufacturer lowercases before lookup.
var manufacturerAliases = map[string]string{
	// Flock Safety — "Flock Group Inc." is the legal entity name.
	"flock":             "Flock Safety",
	"flock safety":      "Flock Safety",
	"flock safety inc":  "Flock Safety",
	"flock safety inc.": "Flock Safety",
	"flock group inc":   "Flock Safety",
	"flock group inc.":  "Flock Safety",

	// Motorola acquired Vigilant Solutions, so tags carry both names.
	"motorola":                      "Motorola Solutions",
	"motorola solutions":            "Motorola Solutions",
	"motorola solutions(vigilant)":  "Motorola Solutions",
	"motorola solutions (vigilant)": "Motorola Solutions",
	"vigilant":                      "Motorola Solutions",
	"vigilant solutions":            "Motorola Solutions",

	"genetec": "Genetec",

	"axis":                "Axis Communications",
	"axis communications": "Axis Communications",

	"leonardo": "Leonardo",
	"leonardo us cyber and security solutions, llc":  "Leonardo",
	"leonardo us cyber and security solutions llc":   "Leonardo",
	"leonardo us cyber and security solutions, inc.": "Leonardo",
	"leonardo us cyber and security solutions inc":   "Leonardo",

	"rekor":              "Rekor",
	"rekor systems":      "Rekor",
	"rekor systems inc":  "Rekor",
	"rekor systems inc.": "Rekor",

	"ubicquia":       "Ubicquia",
	"ubicquia, inc":  "Ubicquia",
	"ubicquia, inc.": "Ubicquia",
	"ubicquia inc":   "Ubicquia",

	"axon":            "Axon",
	"axon enterprise": "Axon",

	"neology":       "Neology",
	"neology, inc.": "Neology",
	"neology inc":   "Neology",

	"verkada":      "Verkada",
	"verkada inc":  "Verkada",
	"verkada inc.": "Verkada",

	"avigilon": "Avigilon",

	// "Mobitix" is a common misspelling of Mobotix.
	"mobotix": "Mobotix",
	"mobitix": "Mobotix",

	"bosch":                  "Bosch",
	"bosch security systems": "Bosch",

	"uniview":              "Uniview",
	"uniview technologies": "Uniview",

	"kapsch":            "Kapsch",
	"kapsch vrx-350x":   "Kapsch",
	"kapsch trafficcom": "Kapsch",

	// "Pacetalk" is a misspelling of Packetalk.
	"packetalk": "Packetalk",
	"pacetalk":  "Packetalk",

	"liveview technologies": "LiveView Technologies",
	"lvt":                   "LiveView Technologies",

	"platesmart":                "PlateSmart",
	"platesmart/cyclopstchnlgs": "PlateSmart",
	"cyclopstechnologies":       "PlateSmart",
	"cyclops technologies":      "PlateSmart",

	"redspeed":         "RedSpeed",
	"redspeed usa":     "RedSpeed",
	"redspeed redcurb": "RedSpeed",

	"hikvision": "Hikvision",
	"dahua":     "Dahua",
	"reconyx":   "Reconyx",
	"transcore": "TransCore",
	"iteris":    "Iteris",
	"costar":    "CoStar",
	"ekin":      "Ekin",
	// Product names rather than company names.
	"ekin box spotter": "Ekin",
	"ekin x spotter":   "Ekin",
}

// Values that mean "we don't know". Stored as NULL rather than becoming a
// literal vendor named "Unknown" or "other" in the filter list.
var manufacturerUnknown = map[string]bool{
	"unknown":     true,
	"unkwn":       true,
	"unspecified": true,
	"other":       true,
	"n/a":         true,
	"na":          true,
	"none":        true,
	"?":           true,
	"-":           true,
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

// NormalizeOperator maps OSM's `operator` tag to the entity that runs the
// camera, or nil when the tag doesn't identify one.
//
// The important case: the single most common `operator` value in the data
// is "Flock Safety" — the vendor, not an agency. Treating that as an
// operator would manufacture a "Flock Safety" deployment record in every
// state and attribute police deployments to a supplier. Anything that
// normalizes to a known manufacturer is therefore rejected outright.
//
// Operators that are genuinely not law enforcement (retailers, universities,
// HOAs) are kept as-is. They're real deployments and belong in the data; a
// moderator decides how to characterise them, and guessing here would only
// hide them.
func NormalizeOperator(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	key := strings.ToLower(trimmed)
	if manufacturerUnknown[key] {
		return nil
	}
	// A vendor name in the operator field tells us nothing about who runs it.
	if _, isVendor := manufacturerAliases[key]; isVendor {
		return nil
	}

	return &trimmed
}
