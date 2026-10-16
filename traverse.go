package typer

import "github.com/corego/typer/internal/unwrap"

// Unwrap returns the single successor of v and reports whether v exposed one,
// i.e. whether v implements Unwrapper[T]. See [unwrap.Unwrap] for the full
// semantics.
func Unwrap[T any](v T) (T, bool) { return unwrap.Unwrap(v) }

// UnwrapAll returns the successors of v and reports whether v exposed any. See
// [unwrap.UnwrapAll] for the full semantics.
func UnwrapAll[T any](v T) ([]T, bool) { return unwrap.UnwrapAll(v) }

// Walk visits every reachable node rooted at root in pre-order, depth-first
// order, calling visit on each. See [unwrap.Walk] for the full semantics.
func Walk[T any](root T, visit func(T) bool) bool { return unwrap.Walk(root, visit) }
