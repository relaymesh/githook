package core

import "testing"

func TestLikePatternToRegex(t *testing.T) {
	if got := likePatternToRegex("%main_"); got != "^.*main.$" {
		t.Fatalf("unexpected regex: %q", got)
	}
	if got := likePatternToRegex("a.b"); got != "^a\\.b$" {
		t.Fatalf("expected escaped regex, got %q", got)
	}
}

func TestSliceContainsAndReflectContains(t *testing.T) {
	if !sliceContains([]interface{}{1, "a", true}, "a") {
		t.Fatalf("expected sliceContains true")
	}
	if sliceContains([]interface{}{1, 2}, "x") {
		t.Fatalf("expected sliceContains false")
	}

	if !reflectContains([]string{"x", "y"}, "y") {
		t.Fatalf("expected reflectContains true for slice")
	}
	if !reflectContains(map[string]int{"x": 1}, "x") {
		t.Fatalf("expected reflectContains true for map key")
	}
	if reflectContains(123, 1) {
		t.Fatalf("expected reflectContains false for unsupported kind")
	}
}
