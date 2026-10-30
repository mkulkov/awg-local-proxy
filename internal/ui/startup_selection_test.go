package ui

import (
	"github.com/mkulkov/awg-local-proxy/internal/profiles"
	"testing"
)

func TestSavedProfileSelectionNeverSubstitutesByName(t *testing.T) {
	items := []profiles.Metadata{{ID: "first", Name: "same"}, {ID: "demo", Name: "same", Demo: true}, {ID: "wanted", Name: "same"}}
	if savedProfileIndex(items, "wanted") != 2 {
		t.Fatal("saved ID did not select exact non-demo profile")
	}
	for _, id := range []string{"", "missing", "demo"} {
		if savedProfileIndex(items, id) != -1 {
			t.Fatalf("unavailable saved profile %q was substituted", id)
		}
	}
}
