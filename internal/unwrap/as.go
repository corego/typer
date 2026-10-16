package unwrap

// matchNode applies the per-node As matching rules and returns the matched
// value. The rules are tried in order: a direct assertion to E, the typed
// Aser[E] override, and finally the legacy interface{ As(any) bool } override.
// It returns the zero value of E and false when none match.
func matchNode[E any, T any](v T) (E, bool) {
	if e, ok := any(v).(E); ok {
		return e, true
	}
	if a, ok := any(v).(Aser[E]); ok {
		var out E
		if a.As(&out) {
			return out, true
		}
	}
	if a, ok := any(v).(interface{ As(any) bool }); ok {
		var out E
		if a.As(&out) {
			return out, true
		}
	}
	var zero E
	return zero, false
}

// As searches the graph rooted at root for the first value assignable to E and
// returns it. The edge type T is inferred from root; the match type E is
// supplied explicitly via a partial type argument, e.g. As[*MyErr](err) with T
// inferred as error.
//
// At each node, in order, As tries: a direct type assertion to E; the typed
// override trait Aser[E]; and finally the standard-library-compatible override
// interface{ As(any) bool }. If none match, the traversal descends into the
// node's children. It returns the zero value of E and false when no match is
// found.
func As[E any, T any](root T) (E, bool) {
	var out E
	found := false
	walkNodes(root, func(v T) bool {
		if e, ok := matchNode[E](v); ok {
			out, found = e, true
			return true
		}
		return false
	})
	if !found {
		var zero E
		return zero, false
	}
	return out, true
}

// AsFrom searches a graph for the first value assignable to E when the root is
// stored as any, or when the root merely holds a value of the edge type T as a
// field. The match type E and edge type T are both supplied explicitly, e.g.
// AsFrom[Fsx, Fs](root).
//
// Because a root passed through an any does not statically implement T, the
// typed walk As cannot seed from it directly. AsFrom therefore inspects the
// root, in order, to find a seed for that walk:
//
//  1. If root (non-nil) is itself assignable to E it is returned directly.
//  2. Else if root is assignable to T, the typed traversal As[E] is seeded
//     from it.
//  3. Else if root implements Unwrapper[T], As[E] is seeded from its child.
//  4. Else if root implements MultiUnwrapper[T], each child is tried in order
//     with As[E] and the first match wins.
//
// Only the first node benefits from this any-typed seed; once a seed of type T
// is found the remainder of the walk is the ordinary typed As. AsFrom returns
// the zero value of E and false when root is nil or no seed matches.
//
// The motivating case is a wrapper that is not itself the edge type but holds
// one as a field:
//
//	type Fs interface{ Open() }
//	type Fsx interface{ Get() }
//
//	type realFs struct{}
//	func (realFs) Open() {}
//	func (realFs) Get()  {}
//
//	type proxyfs struct{ fs Fs }
//	func (p *proxyfs) Unwrap() Fs { return p.fs }
//
//	fsx, ok := AsFrom[Fsx, Fs](&proxyfs{fs: realFs{}})
//	// ok == true; fsx is the realFs found via the seed step, even though
//	// *proxyfs does not itself implement Fs.
func AsFrom[E any, T any](root any) (E, bool) {
	var zero E
	if root == nil {
		return zero, false
	}
	if e, ok := root.(E); ok {
		return e, true
	}
	if t, ok := root.(T); ok {
		return As[E](t)
	}
	if u, ok := root.(Unwrapper[T]); ok {
		return As[E](u.Unwrap())
	}
	if mu, ok := root.(MultiUnwrapper[T]); ok {
		for _, c := range mu.Unwrap() {
			if e, ok := As[E](c); ok {
				return e, true
			}
		}
	}
	return zero, false
}
