package config

import "testing"

// The site is reachable at several origins (custom domain, www, and the
// Cloud Run URL behind it). A single-origin CORS config silently blocks
// every data request from the others — the map and record list just come
// back empty, with no server-side error to notice.
func TestAllowedOrigins(t *testing.T) {
	cases := map[string][]string{
		"https://theflockwatcher.com": {"https://theflockwatcher.com"},
		"https://theflockwatcher.com,https://www.theflockwatcher.com": {
			"https://theflockwatcher.com", "https://www.theflockwatcher.com",
		},
		// Whitespace and trailing commas are easy to introduce in an
		// environment variable and must not become empty origins.
		" https://a.example , https://b.example ,": {"https://a.example", "https://b.example"},
		"":   nil,
		",,": nil,
	}

	for in, want := range cases {
		got := Config{AllowedOrigin: in}.AllowedOrigins()
		if len(got) != len(want) {
			t.Errorf("AllowedOrigins(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("AllowedOrigins(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}
