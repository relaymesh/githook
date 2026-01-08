package core

import "testing"

// TestFlattenNestedAndArray tests that a nested map with an array is flattened correctly.
func TestFlattenNestedAndArray(t *testing.T) {
	input := map[string]interface{}{
		"pull_request": map[string]interface{}{
			"draft": false,
			"commits": []interface{}{
				map[string]interface{}{"created": true},
				map[string]interface{}{"created": false},
			},
		},
	}

	flat := Flatten(input)
	if flat["pull_request.draft"] != false {
		t.Fatalf("expected pull_request.draft to be false")
	}
	if _, ok := flat["pull_request.commits[]"]; !ok {
		t.Fatalf("expected pull_request.commits[] to exist")
	}
	if flat["pull_request.commits[0].created"] != true {
		t.Fatalf("expected commits[0].created to be true")
	}
	if flat["pull_request.commits[1].created"] != false {
		t.Fatalf("expected commits[1].created to be false")
	}
}
