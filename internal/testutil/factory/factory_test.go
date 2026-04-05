package factory_test

import (
	"fmt"
	"testing"

	"github.com/mikecsmith/ihj/internal/testutil/factory"
)

type demo struct {
	ID   int64
	Name string
}

func newDemoFactory() *factory.Factory[demo] {
	return factory.New(func(seq int64) demo {
		return demo{ID: seq, Name: fmt.Sprintf("demo-%d", seq)}
	})
}

func TestFactory_BuildProducesSequentialIDs(t *testing.T) {
	f := newDemoFactory()
	a := f.Build()
	b := f.Build()
	c := f.Build()
	if a.ID != 1 || b.ID != 2 || c.ID != 3 {
		t.Fatalf("ids = %d,%d,%d; want 1,2,3", a.ID, b.ID, c.ID)
	}
}

func TestFactory_TraitsAppliedInOrder(t *testing.T) {
	f := newDemoFactory()
	upper := func(d *demo) { d.Name = "A-" + d.Name }
	suffix := func(d *demo) { d.Name += "-Z" }
	got := f.Build(upper, suffix)
	if got.Name != "A-demo-1-Z" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestFactory_BuildListLength(t *testing.T) {
	f := newDemoFactory()
	items := f.BuildList(5)
	if len(items) != 5 {
		t.Fatalf("len = %d", len(items))
	}
	for i, it := range items {
		if it.ID != int64(i+1) {
			t.Fatalf("items[%d].ID = %d", i, it.ID)
		}
	}
}

func TestFactory_ResetRewindsSequence(t *testing.T) {
	f := newDemoFactory()
	_ = f.BuildList(3)
	f.Reset()
	got := f.Build()
	if got.ID != 1 {
		t.Fatalf("after Reset, ID = %d; want 1", got.ID)
	}
}

func TestFactory_PeekReturnsLastSequence(t *testing.T) {
	f := newDemoFactory()
	if f.Peek() != 0 {
		t.Fatalf("initial Peek = %d; want 0", f.Peek())
	}
	_ = f.BuildList(4)
	if f.Peek() != 4 {
		t.Fatalf("Peek = %d; want 4", f.Peek())
	}
}
