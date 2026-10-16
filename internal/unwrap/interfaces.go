// Package unwrap provides the implementation of the generic "trait"
// generalization of the standard library's errors.Unwrap, errors.Is and
// errors.As primitives.
//
// Where the standard library fixes the chain-link type to the concrete error
// interface, this package parameterizes it as an arbitrary edge type T. A node
// of type T may expose its successors by implementing Unwrapper[T] (a single
// child, mirroring the optional Unwrap() error method) or MultiUnwrapper[T] (a
// slice of children, mirroring the Unwrap() []error convention used by
// errors.Join).
//
// Matching is optional and opt-in. A node may override the default behaviour by
// implementing Iser[T] (for sentinel identity queries) or Aser[E] (for typed
// extraction). When these overrides are absent the primitives fall back to the
// same semantics as the standard library: equality for Is and type assertion
// for As.
package unwrap

// Unwrapper is the generic analogue of the optional "Unwrap() error" method. A
// value of type T that implements Unwrapper[T] exposes exactly one successor of
// the same edge type, allowing a chain to be walked one link at a time.
type Unwrapper[T any] interface {
	// Unwrap returns the single successor node. It may return the zero value of
	// T to signal the end of the chain.
	Unwrap() T
}

// MultiUnwrapper is the generic analogue of the optional "Unwrap() []error" method,
// as used by the standard library's multi-error support. A value of type T that
// implements MultiUnwrapper[T] exposes an ordered slice of successor nodes.
type MultiUnwrapper[T any] interface {
	// Unwrap returns the ordered successors. It may return nil or an empty slice
	// to signal that there are no children.
	Unwrap() []T
}

// Iser is an optional override that lets a node answer a sentinel query without
// relying on equality. It mirrors the optional "Is(error) bool" method honoured
// by the standard library's errors.Is.
type Iser[T comparable] interface {
	// Is reports whether the node should be considered a match for target.
	Is(T) bool
}

// Aser is an optional override that lets a node populate a caller-provided
// output value, mirroring the optional "As(any) bool" method honoured by the
// standard library's errors.As. Implementations should only assign to out and
// return true when the node matches E.
type Aser[T any] interface {
	// As writes the matched value through out and reports whether a match
	// occurred.
	As(*T) bool
}

// MaxDepth bounds the number of edges traversed by the walking primitives, which
// guarantees termination even for self-referential or cyclic graphs. Because it
// bounds depth rather than total work, graphs whose nodes share children can
// still be expensive to traverse.
const MaxDepth = 1_000
