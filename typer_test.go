package typer

import (
	"errors"
	"testing"
)

type leafErr struct{ msg string }

func (e *leafErr) Error() string { return e.msg }

type wrapErr struct {
	msg string
	err error
}

func (e *wrapErr) Error() string { return e.msg }

func (e *wrapErr) Unwrap() error { return e.err }

type multiErr struct {
	msg  string
	errs []error
}

func (m *multiErr) Error() string { return m.msg }

func (m *multiErr) Unwrap() []error { return m.errs }

type chainErr struct {
	tag  string
	next error
}

func (e *chainErr) Error() string { return e.tag }

func (e *chainErr) Unwrap() error { return e.next }

type sliceErr struct{ s []int }

func (e sliceErr) Error() string { return "slice" }

type iserErr struct{ key int }

func (e *iserErr) Error() string { return "iser" }

func (e *iserErr) Is(target error) bool {
	t, ok := target.(*iserErr)
	return ok && t.key == e.key
}

type aserNode struct{ n int }

func (a *aserNode) As(out *int) bool {
	*out = a.n
	return true
}

type legacyErr struct{ inner error }

func (l *legacyErr) Error() string { return "legacy" }

func (l *legacyErr) As(target any) bool {
	if t, ok := target.(**leafErr); ok {
		if le, ok := l.inner.(*leafErr); ok {
			*t = le
			return true
		}
	}
	return false
}

type selfErr struct{ msg string }

func (e *selfErr) Error() string { return e.msg }

func (e *selfErr) Unwrap() error { return e }

type cycErr struct {
	tag  string
	next *cycErr
}

func (e *cycErr) Error() string { return e.tag }

func (e *cycErr) Unwrap() error { return e.next }

type link struct {
	n    int
	next *link
}

func (l *link) Unwrap() *link { return l.next }

func buildChain(n int) *link {
	var head *link
	for i := n; i > 0; i-- {
		head = &link{n: i, next: head}
	}
	return head
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAsSelfMatchAtDepthZeroWins(t *testing.T) {
	first := &chainErr{tag: "first"}
	second := &chainErr{tag: "second"}
	first.next = second

	got, ok := As[*chainErr](error(first))
	if !ok {
		t.Fatalf("As returned ok=false, want true")
	}
	if got != first {
		t.Fatalf("As returned %p, want root %p (pre-order depth 0)", got, first)
	}
}

func TestAsFindsDeeperAndReportsAbsent(t *testing.T) {
	inner := &leafErr{msg: "inner"}
	root := &wrapErr{msg: "root", err: inner}

	got, ok := As[*leafErr](error(root))
	if !ok || got != inner {
		t.Fatalf("As got (%v, %v), want (%p, true)", got, ok, inner)
	}

	missing, ok := As[*leafErr](errors.New("plain"))
	if ok {
		t.Fatalf("As reported ok=true for absent target, got %v", missing)
	}
	if missing != nil {
		t.Fatalf("As returned non-zero %v for absent target", missing)
	}
}

func TestAsOverMultiUnwrapperFirstBranchWins(t *testing.T) {
	a := &leafErr{msg: "a"}
	b := &leafErr{msg: "b"}
	root := &multiErr{msg: "multi", errs: []error{a, b}}

	got, ok := As[*leafErr](error(root))
	if !ok {
		t.Fatalf("As returned ok=false, want true")
	}
	if got != a {
		t.Fatalf("As returned %p, want first branch %p", got, a)
	}
}

func TestAsErrorReturnsRoot(t *testing.T) {
	root := errors.New("boom")
	got, ok := As[error](root)
	if !ok || got != root {
		t.Fatalf("As[error] got (%v, %v), want (%v, true)", got, ok, root)
	}
}

func TestAsErrorFindsConcreteInChainAndTree(t *testing.T) {
	inner := &leafErr{msg: "inner"}
	chain := &wrapErr{msg: "wrap", err: inner}
	got, ok := AsError[*leafErr](chain)
	if !ok || got != inner {
		t.Fatalf("AsError chain got (%v, %v), want (%p, true)", got, ok, inner)
	}

	joinedTarget := &leafErr{msg: "joined"}
	joined := errors.Join(errors.New("x"), error(joinedTarget))
	got2, ok2 := AsError[*leafErr](joined)
	if !ok2 || got2 != joinedTarget {
		t.Fatalf("AsError join got (%v, %v), want (%p, true)", got2, ok2, joinedTarget)
	}
}

func TestAsErrorAbsent(t *testing.T) {
	got, ok := AsError[*leafErr](errors.New("plain"))
	if ok {
		t.Fatalf("AsError reported ok=true, got %v", got)
	}
	if got != nil {
		t.Fatalf("AsError returned non-zero %v", got)
	}
}

func TestIsSentinelIdentityNotText(t *testing.T) {
	sentinel := errors.New("sentinel")
	root := &wrapErr{msg: "wrap", err: sentinel}
	if !Is[error](root, sentinel) {
		t.Fatalf("Is did not find sentinel in chain")
	}
	distinct := errors.New("sentinel")
	if Is[error](root, distinct) {
		t.Fatalf("Is matched a distinct value with equal text")
	}
}

func TestIsFindsSentinelInMultiTree(t *testing.T) {
	sentinel := errors.New("sentinel")
	joined := errors.Join(errors.New("other"), sentinel)
	if !Is[error](joined, sentinel) {
		t.Fatalf("Is did not find sentinel in joined tree")
	}
}

func TestIsNonComparableDynamicTypeNoPanic(t *testing.T) {
	a := error(sliceErr{s: []int{1}})
	b := error(sliceErr{s: []int{2}})
	if Is[error](a, b) {
		t.Fatalf("Is matched non-comparable distinct values")
	}
}

func TestIsNonComparableIdentityNoPanic(t *testing.T) {
	a := error(sliceErr{s: []int{1}})
	if Is[error](a, a) {
		t.Fatalf("Is matched non-comparable identity (expected false, no panic)")
	}
}

func TestIsSelfCycleTerminates(t *testing.T) {
	sc := &selfErr{msg: "self"}
	if !Is[error](sc, sc) {
		t.Fatalf("Is did not self-match in cycle")
	}
	if Is[error](sc, errors.New("absent")) {
		t.Fatalf("Is matched absent target in self-cycle")
	}
}

func TestIsTwoNodeCycleTerminates(t *testing.T) {
	a := &cycErr{tag: "a"}
	b := &cycErr{tag: "b"}
	a.next = b
	b.next = a
	if Is[error](a, errors.New("absent")) {
		t.Fatalf("Is matched absent target in two-node cycle")
	}
}

type concreteErr struct{ msg string }

func (e *concreteErr) Error() string { return e.msg }

var sentinel = errors.New("sentinel")

type nu struct{}

func (n *nu) Error() string { return "nu" }

func (n *nu) Unwrap() error { return sentinel }

type alwaysIser struct{}

func (a *alwaysIser) Error() string { return "always" }

func (a *alwaysIser) Is(error) bool { return true }

type boxErr struct{ X any }

func (b boxErr) Error() string { return "box" }

func TestIsTypedNilChildUnwrapParity(t *testing.T) {
	root := &wrapErr{msg: "root", err: (*nu)(nil)}
	if !Is[error](error(root), sentinel) {
		t.Fatalf("Is did not traverse typed-nil child and match sentinel")
	}
}

func TestAsErrorTypedNilMatchParity(t *testing.T) {
	got, ok := AsError[*concreteErr](error((*concreteErr)(nil)))
	if !ok {
		t.Fatalf("AsError returned ok=false, want true for typed-nil match")
	}
	if got != nil {
		t.Fatalf("AsError returned %v, want nil *concreteErr", got)
	}
}

func TestIsNilTargetParity(t *testing.T) {
	if !Is[error](nil, nil) {
		t.Fatalf("Is[error](nil, nil) = false, want true")
	}
	root := errors.New("root")
	if Is[error](root, nil) {
		t.Fatalf("Is[error](root, nil) = true for non-nil root, want false")
	}
	if Is[error](&alwaysIser{}, nil) {
		t.Fatalf("Is consulted Iser override for nil target, want false")
	}
}

func TestIsNonComparableAggregateNoPanic(t *testing.T) {
	a := error(boxErr{X: []int{1}})
	b := error(boxErr{X: []int{1}})
	if Is[error](a, b) {
		t.Fatalf("Is matched non-comparable aggregate values")
	}
}

func TestWalkPreOrderAndEarlyStop(t *testing.T) {
	root := &multiErr{msg: "m", errs: []error{
		&leafErr{msg: "a"},
		&wrapErr{msg: "w", err: &leafErr{msg: "b"}},
	}}

	var order []string
	stopped := Walk[error](root, func(e error) bool {
		order = append(order, e.Error())
		return e.Error() == "w"
	})
	if !stopped {
		t.Fatalf("Walk returned stopped=false, want true")
	}
	if want := []string{"m", "a", "w"}; !equalStrings(order, want) {
		t.Fatalf("Walk order = %v, want %v", order, want)
	}

	completed := Walk[error](root, func(error) bool { return false })
	if completed {
		t.Fatalf("Walk returned stopped=true for full traversal, want false")
	}
}

func TestUnwrapAllOrderAndEmpty(t *testing.T) {
	a := &leafErr{msg: "a"}
	b := &leafErr{msg: "b"}
	multi := &multiErr{msg: "m", errs: []error{a, b}}

	got, ok := UnwrapAll[error](error(multi))
	if !ok {
		t.Fatalf("UnwrapAll returned ok=false, want true")
	}
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("UnwrapAll = %v, want [a b]", got)
	}

	empty := &multiErr{msg: "e"}
	gotEmpty, okEmpty := UnwrapAll[error](error(empty))
	if !okEmpty {
		t.Fatalf("UnwrapAll empty returned ok=false, want true")
	}
	if gotEmpty != nil {
		t.Fatalf("UnwrapAll empty = %v, want nil", gotEmpty)
	}

	wrap := &wrapErr{msg: "w", err: a}
	gotSingle, okSingle := UnwrapAll[error](error(wrap))
	if !okSingle || len(gotSingle) != 1 || gotSingle[0] != a {
		t.Fatalf("UnwrapAll single = %v, want [a]", gotSingle)
	}

	single, okSingleUnwrap := Unwrap[error](error(wrap))
	if !okSingleUnwrap || single != a {
		t.Fatalf("Unwrap = (%v, %v), want (%v, true)", single, okSingleUnwrap, a)
	}
	if _, ok := Unwrap[error](error(a)); ok {
		t.Fatalf("Unwrap returned ok=true for non-unwrapper")
	}
}

func TestIsIserOverride(t *testing.T) {
	x := &iserErr{key: 7}
	y := &iserErr{key: 7}
	if !Is[error](x, y) {
		t.Fatalf("Iser override did not match non-identical equal keys")
	}
	z := &iserErr{key: 8}
	if Is[error](x, z) {
		t.Fatalf("Iser override matched differing keys")
	}
}

func TestAsAserOverrideWritesThroughPointer(t *testing.T) {
	v := &aserNode{n: 42}
	got, ok := As[int](v)
	if !ok || got != 42 {
		t.Fatalf("As[int] got (%v, %v), want (42, true)", got, ok)
	}
}

func TestAsErrorHonorsLegacyAsOverride(t *testing.T) {
	target := &leafErr{msg: "deep"}
	le := &legacyErr{inner: target}
	got, ok := AsError[*leafErr](le)
	if !ok || got != target {
		t.Fatalf("AsError legacy got (%v, %v), want (%p, true)", got, ok, target)
	}
}

func buildErrChain(n int) error {
	var e error = sentinel
	for i := 0; i < n; i++ {
		e = &wrapErr{msg: "w", err: e}
	}
	return e
}

type Fs interface{ Open() }

type Fsx interface{ Get() }

type realFs struct{ tag string }

func (realFs) Open() {}

func (realFs) Get() {}

type openFs struct{ child Fs }

func (openFs) Open() {}

func (o openFs) Unwrap() Fs { return o.child }

type proxyfs struct{ fs Fs }

func (p *proxyfs) Unwrap() Fs { return p.fs }

type fsGroup struct{ children []Fs }

func (g fsGroup) Unwrap() []Fs { return g.children }

func TestAsFromProxyfsMotivatingCase(t *testing.T) {
	proxy := &proxyfs{fs: realFs{tag: "real"}}
	fsx, ok := AsFrom[Fsx, Fs](proxy)
	if !ok {
		t.Fatalf("AsFrom returned ok=false, want true")
	}
	got, isReal := fsx.(realFs)
	if !isReal {
		t.Fatalf("AsFrom returned %T, want realFs", fsx)
	}
	if got.tag != "real" {
		t.Fatalf("AsFrom returned fs tag %q, want %q", got.tag, "real")
	}
	fsx.Get()
}

func TestAsFromRootImplementsMatchType(t *testing.T) {
	root := realFs{tag: "direct"}
	got, ok := AsFrom[Fsx, Fs](root)
	if !ok {
		t.Fatalf("AsFrom returned ok=false, want true")
	}
	gotFs, isReal := got.(realFs)
	if !isReal {
		t.Fatalf("AsFrom returned %T, want realFs", got)
	}
	if gotFs.tag != "direct" {
		t.Fatalf("AsFrom returned fs tag %q, want %q", gotFs.tag, "direct")
	}
}

func TestAsFromSeedFromEdgeTypedRoot(t *testing.T) {
	got, ok := AsFrom[Fsx, Fs](openFs{child: realFs{tag: "seeded"}})
	if !ok {
		t.Fatalf("AsFrom returned ok=false, want true")
	}
	gotFs, isReal := got.(realFs)
	if !isReal {
		t.Fatalf("AsFrom returned %T, want realFs", got)
	}
	if gotFs.tag != "seeded" {
		t.Fatalf("AsFrom returned fs tag %q, want %q", gotFs.tag, "seeded")
	}

	if _, ok := AsFrom[Fsx, Fs](realFs{tag: "both"}); !ok {
		t.Fatalf("AsFrom over value implementing both E and T returned ok=false, want true")
	}
}

func TestAsFromUnwrapperNotEdgeType(t *testing.T) {
	proxy := &proxyfs{fs: realFs{tag: "unwrapped"}}
	if _, isFs := any(proxy).(Fs); isFs {
		t.Fatalf("precondition failed: *proxyfs unexpectedly implements Fs")
	}
	got, ok := AsFrom[Fsx, Fs](proxy)
	if !ok {
		t.Fatalf("AsFrom returned ok=false, want true")
	}
	gotFs, isReal := got.(realFs)
	if !isReal {
		t.Fatalf("AsFrom returned %T, want realFs", got)
	}
	if gotFs.tag != "unwrapped" {
		t.Fatalf("AsFrom returned fs tag %q, want %q", gotFs.tag, "unwrapped")
	}
}

func TestAsFromMultiUnwrapperFirstBranch(t *testing.T) {
	group := fsGroup{children: []Fs{
		realFs{tag: "first"},
		realFs{tag: "second"},
	}}
	got, ok := AsFrom[Fsx, Fs](group)
	if !ok {
		t.Fatalf("AsFrom returned ok=false, want true")
	}
	gotFs, isReal := got.(realFs)
	if !isReal {
		t.Fatalf("AsFrom returned %T, want realFs", got)
	}
	if gotFs.tag != "first" {
		t.Fatalf("AsFrom returned fs tag %q, want first branch %q", gotFs.tag, "first")
	}
}

func TestAsFromAbsent(t *testing.T) {
	got, ok := AsFrom[*realFs, Fs](&proxyfs{fs: nil})
	if ok {
		t.Fatalf("AsFrom returned ok=true for nil child, got %v", got)
	}
	if got != nil {
		t.Fatalf("AsFrom returned non-zero %v for nil child", got)
	}

	gotNil, okNil := AsFrom[Fsx, Fs](nil)
	if okNil {
		t.Fatalf("AsFrom(nil) returned ok=true, got %v", gotNil)
	}
	if gotNil != nil {
		t.Fatalf("AsFrom(nil) returned non-zero %v", gotNil)
	}
}

func TestAsFromConsistentWithAs(t *testing.T) {
	root := realFs{tag: "consistent"}
	fromAs, okFrom := AsFrom[Fsx, Fs](root)
	typed, okTyped := As[Fsx](root)
	if okFrom != okTyped {
		t.Fatalf("AsFrom ok=%v, As ok=%v, want equal", okFrom, okTyped)
	}
	if fromAs != typed {
		t.Fatalf("AsFrom %v, As %v, want equal", fromAs, typed)
	}
}

func TestDepthBoundary(t *testing.T) {
	long := buildErrChain(5_000)
	if Is[error](long, sentinel) {
		t.Fatalf("Is matched sentinel beyond MaxDepth")
	}

	inBound := buildErrChain(MaxDepth)
	if !Is[error](inBound, sentinel) {
		t.Fatalf("Is did not find sentinel at depth MaxDepth")
	}

	outOfBound := buildErrChain(MaxDepth + 1)
	if Is[error](outOfBound, sentinel) {
		t.Fatalf("Is matched sentinel at depth MaxDepth+1")
	}

	near := buildChain(5_000)
	count := 0
	Walk[*link](near, func(*link) bool {
		count++
		return false
	})
	if count != MaxDepth+1 {
		t.Fatalf("Walk visited %d nodes, want %d", count, MaxDepth+1)
	}
}
