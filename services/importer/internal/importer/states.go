package importer

// USStates is every state plus DC, queried one at a time rather than as a
// single US-wide bbox: Overpass times out on nationwide queries for a tag
// this common, and per-state batching also gives us the `state` column for
// free without reverse-geocoding each node.
var USStates = []string{
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "DC", "FL", "GA", "HI",
	"ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD", "MA", "MI", "MN",
	"MS", "MO", "MT", "NE", "NV", "NH", "NJ", "NM", "NY", "NC", "ND", "OH",
	"OK", "OR", "PA", "RI", "SC", "SD", "TN", "TX", "UT", "VT", "VA", "WA",
	"WV", "WI", "WY",
}
