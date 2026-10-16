package unwrap

// AsError searches err's chain for the first value assignable to E. It is
// exactly As with the edge type fixed to error, i.e. As[E, error](err), and is
// provided as naming sugar and a documentation anchor for the common case.
//
// As with As, traversal descends through both single-child Unwrap() error
// chains and multi-child Unwrap() []error trees, and preserves errors.As
// compatibility including the legacy interface{ As(any) bool } override. The
// zero value of E and false are returned when no match is found.
func AsError[E error](err error) (E, bool) {
	return As[E, error](err)
}
