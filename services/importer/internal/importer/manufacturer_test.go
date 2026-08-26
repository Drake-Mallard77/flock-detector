package importer

import "testing"

func TestNormalizeManufacturer(t *testing.T) {
	cases := []struct {
		in   string
		want *string
	}{
		// Aliases collapse to one canonical name.
		{"Verkada", strptr("Verkada")},
		{"Verkada Inc.", strptr("Verkada")},
		{"Axon", strptr("Axon")},
		{"Axon Enterprise", strptr("Axon")},
		{"Leonardo", strptr("Leonardo")},
		{"Leonardo US Cyber and Security Solutions, LLC", strptr("Leonardo")},
		{"Motorola", strptr("Motorola Solutions")},
		{"Motorola Solutions", strptr("Motorola Solutions")},

		// Case and surrounding whitespace shouldn't create new vendors.
		{"flock safety", strptr("Flock Safety")},
		{"  Flock Safety  ", strptr("Flock Safety")},

		// "Unknown" is a gap, not a vendor.
		{"", nil},
		{"   ", nil},
		{"Unknown", nil},
		{"unknown", nil},
		{"n/a", nil},

		// Unrecognized vendors pass through rather than being dropped —
		// a new manufacturer should appear, not vanish.
		{"Some New Vendor", strptr("Some New Vendor")},
	}

	for _, c := range cases {
		got := NormalizeManufacturer(c.in)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("NormalizeManufacturer(%q) = %q, want nil", c.in, *got)
		case c.want != nil && got == nil:
			t.Errorf("NormalizeManufacturer(%q) = nil, want %q", c.in, *c.want)
		case c.want != nil && got != nil && *got != *c.want:
			t.Errorf("NormalizeManufacturer(%q) = %q, want %q", c.in, *got, *c.want)
		}
	}
}

func strptr(s string) *string { return &s }
