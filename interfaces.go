// Package typer provides a generic, Rust-style "trait" generalization of the
// standard library's errors.Unwrap, errors.Is and errors.As primitives.
//
// The public API is re-exported from github.com/corego/typer/internal/unwrap.
package typer

import "github.com/corego/typer/internal/unwrap"

// Unwrapper is the generic analogue of the optional "Unwrap() error" method. A
// value of type T that implements Unwrapper[T] exposes exactly one successor of
// the same edge type, allowing a chain to be walked one link at a time.
type Unwrapper[T any] = unwrap.Unwrapper[T]

// MultiUnwrapper is the generic analogue of the optional "Unwrap() []error" method,
// as used by the standard library's multi-error support. A value of type T that
// implements MultiUnwrapper[T] exposes an ordered slice of successor nodes.
type MultiUnwrapper[T any] = unwrap.MultiUnwrapper[T]

// Iser is an optional override that lets a node answer a sentinel query without
// relying on equality. It mirrors the optional "Is(error) bool" method honoured
// by the standard library's errors.Is.
type Iser[T comparable] = unwrap.Iser[T]

// Aser is an optional override that lets a node populate a caller-provided
// output value, mirroring the optional "As(any) bool" method honoured by the
// standard library's errors.As. Implementations should only assign to out and
// return true when the node matches E.
type Aser[T any] = unwrap.Aser[T]

// MaxDepth bounds the number of edges traversed by the walking primitives, which
// guarantees termination even for self-referential or cyclic graphs. Because it
// bounds depth rather than total work, graphs whose nodes share children can
// still be expensive to traverse.
const MaxDepth = unwrap.MaxDepth
