package typer

import "github.com/corego/typer/internal/unwrap"

// AsError searches err's chain for the first value assignable to E. It is
// exactly As with the edge type fixed to error. See [unwrap.AsError] for the
// full semantics.
func AsError[E error](err error) (E, bool) { return unwrap.AsError[E](err) }
