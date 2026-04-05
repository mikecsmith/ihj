package core

import (
	"sort"
	"testing"
)

func TestCompareIDsNatural(t *testing.T) {
	ids := []string{
		"acme/widgets#17",
		"acme/widgets#2",
		"acme/widgets#16",
		"acme/widgets#3",
		"acme/widgets#10",
		"acme/widgets#1",
	}
	sort.Slice(ids, func(i, j int) bool { return compareIDsNatural(ids[i], ids[j]) })
	want := []string{
		"acme/widgets#1",
		"acme/widgets#2",
		"acme/widgets#3",
		"acme/widgets#10",
		"acme/widgets#16",
		"acme/widgets#17",
	}
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("pos %d: got %q want %q (full: %v)", i, ids[i], want[i], ids)
		}
	}
}
