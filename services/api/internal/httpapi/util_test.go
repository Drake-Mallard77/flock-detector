package httpapi

import "testing"

func TestParseBBox_Valid(t *testing.T) {
	west, south, east, north, err := parseBBox("-90,39,-89,40")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if west != -90 || south != 39 || east != -89 || north != 40 {
		t.Errorf("got west=%v south=%v east=%v north=%v", west, south, east, north)
	}
}

func TestParseBBox_Invalid(t *testing.T) {
	cases := []string{
		"",
		"not,a,valid,bbox",
		"1,2,3",
		"1,2,3,4,5",
		"1,2,3,",
	}
	for _, c := range cases {
		if _, _, _, _, err := parseBBox(c); err == nil {
			t.Errorf("parseBBox(%q): expected an error, got none", c)
		}
	}
}
