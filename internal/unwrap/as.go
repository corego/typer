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

// nextBy returns the successors of v exposed as any, consulting the
// single-child Unwrapper[T] trait first and then the multi-child
// MultiUnwrapper[T] trait. It returns nil when v exposes neither. A single type
// cannot implement both traits, because both declare a method named Unwrap, so
// at most one branch fires.
func nextBy[T any](v any) []any {
	if u, ok := v.(Unwrapper[T]); ok {
		return []any{u.Unwrap()}
	}
	if mu, ok := v.(MultiUnwrapper[T]); ok {
		children := mu.Unwrap()
		out := make([]any, len(children))
		for i, c := range children {
			out[i] = c
		}
		return out
	}
	return nil
}

// nextTThenAny returns the successors of v by trying the edge type T and then
// falling back to the universal edge type any. Because a single type cannot
// declare both Unwrap() T and Unwrap() any, the fallback is unambiguous: it
// fires only on nodes that expose no T-typed successor, which a T-only walk
// would otherwise treat as a dead end.
func nextTThenAny[T any](v any) []any {
	if c := nextBy[T](v); c != nil {
		return c
	}
	return nextBy[any](v)
}

// nextABThenAny returns the successors of v by trying the edge types A and B
// in order, then falling back to the universal edge type any.
func nextABThenAny[A any, B any](v any) []any {
	if c := nextBy[A](v); c != nil {
		return c
	}
	if c := nextBy[B](v); c != nil {
		return c
	}
	return nextBy[any](v)
}

// walkAnyNodes performs a pre-order, depth-first traversal over nodes carried
// as any, mirroring walkNodes with the successor edges produced by next. A true
// nil interface and any node deeper than MaxDepth are skipped - neither visited
// nor descended into. Typed-nil nodes ARE visited and descended into.
func walkAnyNodes(root any, next func(any) []any, visit func(any) bool) bool {
	var rec func(v any, depth int) bool
	rec = func(v any, depth int) bool {
		if depth > MaxDepth || v == nil {
			return false
		}
		if visit(v) {
			return true
		}
		for _, c := range next(v) {
			if rec(c, depth+1) {
				return true
			}
		}
		return false
	}
	return rec(root, 0)
}

// asFromMixed is the shared engine behind AsFromWithAny and AsFromWith2. The
// root is visited and matched like any other node; a direct assertion to E is
// tried before the walk begins, mirroring AsFrom. Successors are produced per
// node by next, which may return nil for a leaf.
func asFromMixed[E any](root any, next func(any) []any) (E, bool) {
	if root == nil {
		var zero E
		return zero, false
	}
	if e, ok := root.(E); ok {
		return e, true
	}
	var out E
	found := false
	walkAnyNodes(root, next, func(v any) bool {
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

// AsFromWithAny searches a graph for the first value assignable to E when the
// root is stored as any, or when the root merely holds a value as a field, and
// where the graph's edges may mix the edge type T with the universal edge type
// any. The match type E and primary edge type T are both supplied explicitly,
// e.g. AsFromWithAny[Fsx, Fs](root).
//
// At each node the successors are resolved in order: a single child exposed by
// Unwrapper[T], the children of MultiUnwrapper[T], then the same two traits with
// the edge type any. Because a single type cannot declare both Unwrap() T and
// Unwrap() any, the any fallback is unambiguous: it fires only on nodes that do
// not expose a T-typed successor, which a T-only walk (such as AsFrom) would
// otherwise treat as a dead end. Unlike AsFrom, the root itself is always
// visited and matched.
//
// The motivating case is a chain that mixes nodes returning T with nodes
// returning any:
//
//	type Fs interface{ Open() }
//	type Fsx interface{ Get() }
//
//	type realFs struct{}
//	func (realFs) Open() {}
//	func (realFs) Get()  {}
//
//	type anyFs struct{ child Fs }
//	func (a anyFs) Unwrap() any { return a.child }
//
//	fsx, ok := AsFromWithAny[Fsx, Fs](anyFs{child: realFs{}})
//	// ok == true; AsFrom[Fsx, Fs] would stop at anyFs, whose Unwrap() any is
//	// invisible to a T-only walk.
func AsFromWithAny[E any, T any](root any) (E, bool) {
	return asFromMixed[E](root, nextTThenAny[T])
}

// AsFromWith2 searches a graph for the first value assignable to E when the
// root is stored as any, or when the root merely holds a value as a field, and
// where the graph's edges may mix two edge types A and B, falling back to the
// universal edge type any. The match type E and both edge types are supplied
// explicitly, e.g. AsFromWith2[Fsx, error, Fs](root).
//
// At each node the successors are resolved in order: Unwrapper[A],
// MultiUnwrapper[A], Unwrapper[B], MultiUnwrapper[B], then the same two traits
// with the edge type any. A single type can implement at most one of these
// shapes, so the order never creates ambiguity. This covers graphs whose edges
// mix types invisible to each other, e.g. standard-library Unwrap() error links
// combined with custom Unwrap() Fs links.
func AsFromWith2[E any, A any, B any](root any) (E, bool) {
	return asFromMixed[E](root, nextABThenAny[A, B])
}
