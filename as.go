package typer

import "github.com/corego/typer/internal/unwrap"

// As searches the graph rooted at root for the first value assignable to E and
// returns it. See [unwrap.As] for the full semantics.
func As[E any, T any](root T) (E, bool) { return unwrap.As[E](root) }

// AsFrom searches a graph for the first value assignable to E when the root is
// stored as any, or when the root merely holds a value of the edge type T as a
// field. See [unwrap.AsFrom] for the full semantics.
func AsFrom[E any, T any](root any) (E, bool) { return unwrap.AsFrom[E, T](root) }
