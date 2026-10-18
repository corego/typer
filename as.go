package typer

import "github.com/corego/typer/internal/unwrap"

// As searches the graph rooted at root for the first value assignable to E and
// returns it. See [unwrap.As] for the full semantics.
func As[E any, T any](root T) (E, bool) { return unwrap.As[E](root) }

// AsWithAny searches the graph rooted at root for the first value assignable
// to E when the edges may mix the edge type T with the universal edge type
// any. It is the typed-root form of AsFromWithAny. See [unwrap.AsWithAny] for
// the full semantics.
func AsWithAny[E any, T any](root T) (E, bool) { return unwrap.AsWithAny[E](root) }

// AsFrom searches a graph for the first value assignable to E when the root is
// stored as any, or when the root merely holds a value of the edge type T as a
// field. See [unwrap.AsFrom] for the full semantics.
func AsFrom[E any, T any](root any) (E, bool) { return unwrap.AsFrom[E, T](root) }

// AsFromWithAny searches a graph for the first value assignable to E when the
// root is stored as any and the edges may mix the edge type T with the
// universal edge type any. See [unwrap.AsFromWithAny] for the full semantics.
func AsFromWithAny[E any, T any](root any) (E, bool) { return unwrap.AsFromWithAny[E, T](root) }

// AsFromWith2 searches a graph for the first value assignable to E when the
// root is stored as any and the edges may mix two edge types A and B, falling
// back to the universal edge type any. See [unwrap.AsFromWith2] for the full
// semantics.
func AsFromWith2[E any, A any, B any](root any) (E, bool) { return unwrap.AsFromWith2[E, A, B](root) }
