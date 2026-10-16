package unwrap

// walkNodes performs a pre-order, depth-first traversal of the graph reachable
// from root. At each visited node visit is invoked; returning true from visit
// stops the traversal immediately and propagates true back to the caller. A
// true nil interface and any node deeper than MaxDepth are skipped entirely -
// neither visited nor descended into. Typed-nil nodes (for example a nil
// *myErr stored in an interface) ARE visited and descended into, matching the
// behavior of the standard library's errors.Is and errors.As.
//
// Descent prefers the single-child Unwrapper[T] trait over the multi-child
// MultiUnwrapper[T] trait. A single type cannot implement both, because both
// declare a method named Unwrap.
func walkNodes[T any](root T, visit func(T) bool) bool {
	var rec func(v T, depth int) bool
	rec = func(v T, depth int) bool {
		if depth > MaxDepth || any(v) == nil {
			return false
		}
		if visit(v) {
			return true
		}
		if u, ok := any(v).(Unwrapper[T]); ok {
			return rec(u.Unwrap(), depth+1)
		}
		if mu, ok := any(v).(MultiUnwrapper[T]); ok {
			for _, c := range mu.Unwrap() {
				if rec(c, depth+1) {
					return true
				}
			}
		}
		return false
	}
	return rec(root, 0)
}

// Unwrap returns the single successor of v and reports whether v exposed one,
// i.e. whether v implements Unwrapper[T]. When v does not, it returns the zero
// value of T and false. Multi-child MultiUnwrapper[T] nodes are not consulted.
func Unwrap[T any](v T) (T, bool) {
	if u, ok := any(v).(Unwrapper[T]); ok {
		return u.Unwrap(), true
	}
	var zero T
	return zero, false
}

// UnwrapAll returns the successors of v and reports whether v exposed any. If v
// implements MultiUnwrapper[T] its children are returned as-is; otherwise, if v
// implements Unwrapper[T], a single-element slice wrapping its child is
// returned. When v exposes neither trait it returns (nil, false).
func UnwrapAll[T any](v T) ([]T, bool) {
	if mu, ok := any(v).(MultiUnwrapper[T]); ok {
		return mu.Unwrap(), true
	}
	if u, ok := any(v).(Unwrapper[T]); ok {
		return []T{u.Unwrap()}, true
	}
	return nil, false
}

// Walk visits every reachable node rooted at root in pre-order, depth-first
// order, calling visit on each. Returning true from visit stops the walk early.
//
// The return value reports whether the walk was stopped early: it is true when
// visit returned true, and false when the full traversal ran to completion.
func Walk[T any](root T, visit func(T) bool) bool {
	return walkNodes(root, visit)
}
