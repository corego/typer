package typer

import "github.com/corego/typer/internal/unwrap"

// Is reports whether the graph rooted at root reaches a node that matches
// target by sentinel identity, mirroring the semantics of errors.Is. See
// [unwrap.Is] for the full semantics.
func Is[T comparable](root T, target T) bool { return unwrap.Is(root, target) }
