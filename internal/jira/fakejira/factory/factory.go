// Package factory provides a generic, deterministic factory primitive for
// constructing test/demo objects — inspired by TypeScript's Fishery.
//
// A Factory[T] wraps a builder function that mints a fresh T from an
// integer sequence number. Each call to Build() or BuildList() advances
// the sequence, so a single factory instance produces stable, unique
// values for every call. Callers can apply Traits — small mutating
// closures — to customise individual instances.
//
// Example:
//
//	userFactory := factory.New(func(seq int64) User {
//	    return User{ID: seq, Name: fmt.Sprintf("User %d", seq)}
//	})
//	alice := userFactory.Build(WithName("Alice"))
//	users := userFactory.BuildList(5)
package factory

import "sync/atomic"

// Trait is a mutation applied to a freshly-built T.
type Trait[T any] func(*T)

// Factory builds objects of type T from a deterministic sequence counter.
// The zero value is not usable — construct via New.
type Factory[T any] struct {
	build func(seq int64) T
	seq   atomic.Int64
}

// New creates a Factory that mints T values from the given builder.
// The builder is called with a fresh sequence number on every Build call.
// Sequences start at 1 and increase monotonically per-factory.
func New[T any](build func(seq int64) T) *Factory[T] {
	return &Factory[T]{build: build}
}

// Reset rewinds the sequence counter back to 0 (next Build returns seq 1).
// Useful at the start of a scenario to guarantee deterministic IDs.
func (f *Factory[T]) Reset() {
	f.seq.Store(0)
}

// Peek returns the sequence number of the last item built — or 0 if none yet.
func (f *Factory[T]) Peek() int64 {
	return f.seq.Load()
}

// Build returns a fresh T from the next sequence number, with any traits
// applied in order.
func (f *Factory[T]) Build(traits ...Trait[T]) T {
	n := f.seq.Add(1)
	item := f.build(n)
	for _, t := range traits {
		t(&item)
	}
	return item
}

// BuildList returns n fresh values. Traits apply to every item.
func (f *Factory[T]) BuildList(n int, traits ...Trait[T]) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = f.Build(traits...)
	}
	return out
}
