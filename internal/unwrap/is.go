package unwrap

import "reflect"

// Is reports whether the graph rooted at root reaches a node that matches
// target by sentinel identity, mirroring the semantics of errors.Is. Matching
// is equality-based, not text-based: two distinct values that happen to carry
// the same message do not match.
//
// A nil target matches only a nil root, mirroring errors.Is(err, nil); the
// per-node Iser[T] override is not consulted in that case. Otherwise each node
// is tested in order: value equality, then the optional Iser[T] override.
// Unmatched nodes are descended into via walkNodes.
func Is[T comparable](root T, target T) bool {
	if any(target) == nil {
		return any(root) == nil
	}
	return walkNodes(root, func(v T) bool {
		if isEqualValue(v, target) {
			return true
		}
		if i, ok := any(v).(Iser[T]); ok && i.Is(target) {
			return true
		}
		return false
	})
}

// isEqualValue reports whether a and b are equal without panicking when the
// dynamic value stored in a or b is non-comparable. The comparable constraint
// on T bounds only the static type; a dynamic value such as a struct containing
// an interface field holding a slice would otherwise make == panic. The guard
// therefore inspects the VALUES recursively (reflect.Value.Comparable), not the
// static type alone.
func isEqualValue[T comparable](a, b T) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if !va.IsValid() || !vb.IsValid() {
		return va.IsValid() == vb.IsValid()
	}
	if !vb.Comparable() {
		return false
	}
	return a == b
}
