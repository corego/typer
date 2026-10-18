# API 参考

包 `typer`（导入路径 `github.com/corego/typer`）把标准库的 `errors.Unwrap`、`errors.Is`、`errors.As` 泛化为「边类型可配置」的图遍历原语。本页是完整的符号参考；想看设计动机请读 [design.md](./design.md)，想快速上手请读 [../README.md](../README.md)。

## 目录

- [核心概念](#核心概念)
- [接口](#接口)
- [常量](#常量)
- [函数](#函数)
- [与标准库对照表](#与标准库对照表)
- [基础用法](#基础用法)
- [自定义覆盖](#自定义覆盖)
- [任意边类型](#任意边类型)
- [持有型代理](#持有型代理)
- [已知边界](#已知边界)

## 核心概念

两个类型参数贯穿全库，角色不同，不要混为一谈。

| 名称 | 含义 | 出现位置 |
| --- | --- | --- |
| `T` | 边类型：节点之间连边的类型，决定图长什么样 | `Unwrapper[T]`、`MultiUnwrapper[T]`、`Unwrap[T]`、`UnwrapAll[T]`、`Walk[T]`、`Is[T]`、`As[E, T]`、`AsWithAny[E, T]`、`AsFrom[E, T]`、`AsFromWithAny[E, T]` |
| `E` | 匹配类型：你想从图里捞出来的类型 | `As[E, T]`、`AsWithAny[E, T]`、`AsFrom[E, T]`、`AsFromWithAny[E, T]`、`AsFromWith2[E, A, B]`、`Aser[E]`、`AsError[E]` |

在 `errors.As` 里，`T` 恒为 `error`、`E` 由你指定。本库把 `T` 也开放出来，这就是「异构查找」的含义。

## 接口

### `type Unwrapper[T any] interface { Unwrap() T }`

单子节点接口，泛化版的可选 `Unwrap() error`。

- `Unwrap()` 返回唯一的后继节点，返回类型是边类型 `T`。
- 返回 `T` 的零值表示链到此结束。

### `type MultiUnwrapper[T any] interface { Unwrap() []T }`

多子节点接口，泛化版的 `Unwrap() []error`（标准库 `errors.Join` 采用的那种约定）。

- `Unwrap()` 返回有序的后继切片。
- 返回 `nil` 或空切片表示没有子节点。

### `type Iser[T comparable] interface { Is(T) bool }`

可选的哨兵匹配覆盖，对应标准库认可的 `Is(error) bool`。`Is` 原语会在值相等不成立时尝试调用它。

### `type Aser[T any] interface { As(*T) bool }`

可选的提取覆盖，对应标准库认可的 `As(any) bool`。实现应只在命中时写入 `out` 并返回 `true`。

## 常量

### `const MaxDepth = 1_000`

遍历深度的硬上限。它保证自引用或成环图也能终止，限制的是**深度**而非总工作量：共享子节点的图仍可能遍历成本很高。深度超过 `MaxDepth` 的节点被视为不存在，既不访问也不下探。

## 函数

### `func Unwrap[T any](v T) (T, bool)`

取单子节点的后继。

- 参数 `v`：待检查的节点。
- 返回：若 `v` 实现 `Unwrapper[T]`，返回 `(v.Unwrap(), true)`；否则返回 `(T 的零值, false)`。
- 语义：只认 `Unwrapper[T]`，**不**读 `MultiUnwrapper[T]`。对应标准库 `errors.Unwrap` 的泛化版。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

func main() {
	err := fmt.Errorf("outer: %w", fmt.Errorf("inner"))

	next, ok := typer.Unwrap[error](err)
	fmt.Println(ok)    // true
	fmt.Println(next)  // inner
}
```

### `func UnwrapAll[T any](v T) ([]T, bool)`

取一个节点的全部后继，统一成切片视图。

- 参数 `v`：待检查的节点。
- 返回：若 `v` 实现 `MultiUnwrapper[T]`，原样返回其切片；否则若实现 `Unwrapper[T]`，返回单元素切片 `[]T{child}`；都不满足时返回 `(nil, false)`。
- 语义：把「单子节点链」和「多子节点树」两种形状抹平成同一个切片接口，方便调用方统一处理。

```go
package main

import (
	"errors"
	"fmt"

	"github.com/corego/typer"
)

func main() {
	joined := errors.Join(errors.New("a"), errors.New("b"))

	kids, ok := typer.UnwrapAll[error](joined)
	fmt.Println(ok)       // true
	fmt.Println(len(kids)) // 2
}
```

### `func Walk[T any](root T, visit func(T) bool) bool`

先序、深度优先遍历以 `root` 为起点的可达节点。

- 参数 `root`：遍历起点。
- 参数 `visit`：每个节点的回调。返回 `true` 表示**提前停止**整个遍历。
- 返回：`true` 表示遍历被 `visit` 提前停止；`false` 表示完整跑完。
- 语义：真正的 `nil` 接口与超过 `MaxDepth` 的节点会被跳过；typed-nil 节点（例如装进接口的 `(*myErr)(nil)`）会被访问并下探，与标准库一致。

```go
package main

import (
	"errors"
	"fmt"

	"github.com/corego/typer"
)

func main() {
	err := fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New("c")))

	var msgs []string
	typer.Walk[error](err, func(e error) bool {
		msgs = append(msgs, e.Error())
		return false // false 表示继续遍历
	})
	fmt.Println(len(msgs)) // 3
}
```

### `func As[E any, T any](root T) (E, bool)`

异构查找：在边类型为 `T` 的图里找出第一个可赋值给 `E` 的值。

- 类型参数 `T`：边类型，由 `root` 推导。
- 类型参数 `E`：匹配类型，通过**部分类型实参**显式给出，例如 `As[*MyErr](err)`（此时 `T` 推导为 `error`）。
- 参数 `root`：遍历起点。
- 返回：命中时 `(匹配值, true)`；未命中 `(E 的零值, false)`。
- 每个节点按序尝试：① 直接类型断言 `any(v).(E)`；② 类型化覆盖 `Aser[E]`；③ 兼容标准库的 `interface{ As(any) bool }` 覆盖。命中即返回，否则下探子节点。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type QueryError struct{ Query string }

func (e *QueryError) Error() string { return "bad query: " + e.Query }

func main() {
	err := fmt.Errorf("run: %w", &QueryError{Query: "SELECT"})

	if qerr, ok := typer.As[*QueryError](err); ok {
		fmt.Println(qerr.Query) // SELECT
	}
}
```

### `func AsWithAny[E any, T any](root T) (E, bool)`

`As` 的混合边版：在边类型为 `T` 的图里，节点还可能用通配边类型 `any` 暴露后继。它是 `AsFromWithAny` 的**强类型根**形式，`T` 由 `root` 推导、`E` 显式给出。

- 类型参数 `T`：主边类型，由 `root` 推导。
- 类型参数 `E`：匹配类型，通过**部分类型实参**显式给出，例如 `AsWithAny[*MyErr](err)`（此时 `T` 推导为 `error`）。
- 参数 `root`：遍历起点。
- 返回：命中时 `(匹配值, true)`；未命中 `(E 的零值, false)`。
- 每个节点按序取后继：① `Unwrapper[T]`；② `MultiUnwrapper[T]`；③ 降级 `Unwrapper[any]`；④ 降级 `MultiUnwrapper[any]`。节点匹配规则与 `As` 相同（直接断言 → `Aser[E]` → 旧式 `As(any) bool`）。
- 注意：`T` 由实参的**静态类型**推导。若边类型是接口，请传一个静态类型为该接口的变量（与 `As` 一致），或显式写出两个类型实参，例如 `AsWithAny[Fsx, Fs](root)`。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type Fs interface{ Open() }
type Fsx interface{ Get() }

type realFs struct{ tag string }

func (realFs) Open() {}
func (realFs) Get()  {}

type openFs struct{ child Fs }

func (openFs) Open() {}

func (o openFs) Unwrap() Fs { return o.child }

// fsAny 挂在 Fs 边上（实现 Open），却用 Unwrap() any 暴露后继。
type fsAny struct{ child any }

func (fsAny) Open() {}

func (f fsAny) Unwrap() any { return f.child }

func main() {
	var fs Fs = openFs{child: fsAny{child: realFs{tag: "x"}}}
	fsx, ok := typer.AsWithAny[Fsx](fs)
	fmt.Println(ok) // true：As[Fsx](fs) 会在 fsAny 处停住
	fsx.Get()       // 命中的是 realFs
}
```

### `func AsFrom[E any, T any](root any) (E, bool)`

当 `root` 以 `any` 保存，或 `root` 只是**持有**一个 `T` 字段（自身并不是 `T`）时使用。

- 类型参数 `E`：匹配类型，显式给出。
- 类型参数 `T`：边类型，显式给出（`root` 是 `any`，无法推导）。
- 参数 `root`：起点，类型为 `any`。
- 返回：命中时 `(匹配值, true)`；否则 `(E 的零值, false)`。
- 按序尝试：① `root`（非 nil）本身可赋值给 `E` → 直接返回；② `root` 可赋值给 `T` → 以 `root` 为种子调用 `As[E]`；③ `root` 实现 `Unwrapper[T]` → 以其 `Unwrap()` 为种子调用 `As[E]`；④ `root` 实现 `MultiUnwrapper[T]` → 逐子节点调用 `As[E]`，先命中先返回；⑤ `root == nil` 或都不满足 → `(E 的零值, false)`。
- 语义：只有第一个节点享受 `any` 种子的灵活匹配；一旦拿到类型为 `T` 的种子，后续复用强类型 `As`。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type Fs interface{ Open() }
type Fsx interface{ Get() }

type realFs struct{}

func (realFs) Open() {}
func (realFs) Get()  {}

type proxyfs struct{ fs Fs }

func (p *proxyfs) Unwrap() Fs { return p.fs }

func main() {
	fsx, ok := typer.AsFrom[Fsx, Fs](&proxyfs{fs: realFs{}})
	fmt.Println(ok) // true
	fsx.Get()       // 命中的是 realFs
}
```

### `func AsFromWithAny[E any, T any](root any) (E, bool)`

当 `root` 以 `any` 保存，或只是**持有**一个字段，且图中节点可能混合使用边类型 `T` 与通配边类型 `any` 时使用。是 `AsFrom` 的按节点降级版。

- 类型参数 `E`：匹配类型，显式给出。
- 类型参数 `T`：主边类型，显式给出。
- 参数 `root`：起点，类型为 `any`。
- 返回：命中时 `(匹配值, true)`；否则 `(E 的零值, false)`。
- 每个节点按序取后继：① `Unwrapper[T]`；② `MultiUnwrapper[T]`；③ 降级 `Unwrapper[any]`；④ 降级 `MultiUnwrapper[any]`。
- 因为一个类型无法同时声明 `Unwrap() T` 与 `Unwrap() any`（同名方法不同签名），降级**无歧义**：只对「不暴露 `T` 后继」的节点生效——这些节点在 `AsFrom` 里原本是死胡同。
- 与 `AsFrom` 不同，`AsFromWithAny` 会**访问并匹配根节点本身**（和 `As` 一致），覆盖更彻底。
- 根为 `nil` 返回 `(E 的零值, false)`。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type Fs interface{ Open() }
type Fsx interface{ Get() }

type realFs struct{ tag string }

func (realFs) Open() {}
func (realFs) Get()  {}

// anyFs 用 Unwrap() any 暴露后继；AsFrom[Fsx, Fs] 看不到这条边。
type anyFs struct{ child Fs }

func (a anyFs) Unwrap() any { return a.child }

func main() {
	fsx, ok := typer.AsFromWithAny[Fsx, Fs](anyFs{child: realFs{tag: "x"}})
	fmt.Println(ok) // true
	fsx.Get()       // 命中的是 realFs
}
```

### `func AsFromWith2[E any, A any, B any](root any) (E, bool)`

当图中节点可能混合两种**互不可见**的边类型 `A` 与 `B` 时使用，同样按节点降级到 `any`。典型场景：标准库 `Unwrap() error` 链与自定义 `Unwrap() Fs` 链共存。

- 类型参数 `E`：匹配类型，显式给出。
- 类型参数 `A`、`B`：两种边类型，显式给出。
- 参数 `root`：起点，类型为 `any`。
- 返回：命中时 `(匹配值, true)`；否则 `(E 的零值, false)`。
- 每个节点按序取后继：① `Unwrapper[A]`；② `MultiUnwrapper[A]`；③ `Unwrapper[B]`；④ `MultiUnwrapper[B]`；⑤ 降级 `Unwrapper[any]`；⑥ `MultiUnwrapper[any]`。
- 单个类型至多实现上述一种形状，尝试顺序不会造成歧义。

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type Fs interface{ Open() }
type Fsx interface{ Get() }

type Gs interface{ Run() }

type realGs struct{}

func (realGs) Open() {} // 实现 Fs，可挂在 Fs 边上
func (realGs) Run()  {} // 实现 Gs
func (realGs) Get()  {} // 实现 Fsx，即要找的目标

type openFs struct{ child Fs }

func (openFs) Open() {}

func (o openFs) Unwrap() Fs { return o.child }

// gsNode 挂在 Fs 边上（实现 Open），自身却用 Gs 边暴露后继。
type gsNode struct{ child Gs }

func (gsNode) Open() {}

func (n gsNode) Unwrap() Gs { return n.child }

func main() {
	fsx, ok := typer.AsFromWith2[Fsx, Fs, Gs](openFs{child: gsNode{child: realGs{}}})
	fmt.Println(ok) // true
	fsx.Get()
}
```

### `func Is[T comparable](root T, target T) bool`

哨兵相等判定，语义与 `errors.Is` 一致，不按文本比较。

- 参数 `root`：遍历起点。
- 参数 `target`：要找的哨兵值。
- 返回：图中任一行与 `target` 相等即 `true`。
- 语义：`target` 为 nil 时，仅当 `root` 也为 nil 才返回 `true`（此时**不**调用 `Iser` 覆盖）。否则每个节点按序：① 值相等；② `Iser[T]` 覆盖。
- 比较有防 panic 保护：`T` 的 `comparable` 约束只约束静态类型，动态值若含不可比较成分（如内嵌 interface 持有切片的结构体），不会 panic，只是判为不相等。

```go
package main

import (
	"errors"
	"fmt"

	"github.com/corego/typer"
)

func main() {
	sentinel := errors.New("boom")
	err := fmt.Errorf("wrap: %w", sentinel)

	fmt.Println(typer.Is[error](err, sentinel)) // true（同一实例）
	fmt.Println(typer.Is[error](err, errors.New("boom"))) // false（仅文本相同）
}
```

### `func AsError[E error](err error) (E, bool)`

就是 `As[E, error](err)`，边类型固定为 `error`。作为常见场景的命名糖与文档锚点，同时保留 `errors.As` 兼容性（含旧的 `interface{ As(any) bool }` 覆盖）。

```go
package main

import (
	"errors"
	"fmt"

	"github.com/corego/typer"
)

type myErr struct{}

func (*myErr) Error() string { return "my" }

func main() {
	err := fmt.Errorf("wrap: %w", &myErr{})

	var target *myErr
	if e, ok := typer.AsError[*myErr](err); ok {
		target = e
	}
	fmt.Println(target != nil) // true
	fmt.Println(errors.As(err, &target)) // true（与标准库一致）
}
```

## 与标准库对照表

| 标准库 | 本库 | 说明 |
| --- | --- | --- |
| `errors.Unwrap(err)` | `Unwrap[error](err)` | 单子节点，取一个后继 |
| `errors.Is(err, target)` | `Is[error](err, target)` | 哨兵相等，支持 `Iser` 覆盖 |
| `errors.As(err, &target)` | `AsError[E](err)` / `As[E, error](err)` | 异构提取，支持 `Aser` 与旧式 `As(any) bool` 覆盖 |
| `errors.Join(errs...)` 及 `Unwrap() []error` | `MultiUnwrapper[error]` | 多子节点树式遍历 |

## 基础用法

最常见的场景就是把本库当成标准库 `errors` 的平替。边类型取 `error`，其余调用形态几乎不变：

```go
err := fmt.Errorf("run: %w", &QueryError{Query: "SELECT"})

// 找类型
if qerr, ok := typer.AsError[*QueryError](err); ok {
	_ = qerr
}

// 找哨兵
if typer.Is[error](err, io.EOF) {
	// ...
}

// 看下一个链节
if next, ok := typer.Unwrap[error](err); ok {
	_ = next
}
```

## 自定义覆盖

### `Iser[T]`：按业务含义匹配

当哨兵相等不适用时，让节点自己回答「是否匹配」：

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type codeErr struct{ code int }

func (e *codeErr) Error() string { return "code err" }

func (e *codeErr) Is(target error) bool {
	t, ok := target.(*codeErr)
	return ok && t.code == e.code
}

func main() {
	wrapped := fmt.Errorf("op: %w", &codeErr{code: 42})
	fmt.Println(typer.Is[error](wrapped, &codeErr{code: 42})) // true
}
```

### `Aser[E]`：自行决定如何写回

节点可以实现类型化的 `As(*E) bool`，把匹配值写进调用方指针：

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type box struct{ v int }

func (b *box) As(out *int) bool {
	*out = b.v
	return true
}

func main() {
	got, ok := typer.As[int](&box{v: 7})
	fmt.Println(got, ok) // 7 true
}
```

## 任意边类型

`T` 不限于 `error`。任何能通过 `Unwrapper[T]` 或 `MultiUnwrapper[T]` 连起来的结构都能用同一套原语。下面是一条 `*link` 链：

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type link struct {
	name string
	next *link
}

func (l *link) Unwrap() *link { return l.next }

func main() {
	root := &link{name: "a", next: &link{name: "b", next: &link{name: "c"}}}

	n := 0
	typer.Walk[*link](root, func(*link) bool {
		n++
		return false // false 表示继续
	})
	fmt.Println(n) // 3
}
```

`Walk[*link]`、`Is[*link]`、`As[...]` 在任意边类型上行为一致，只是把 `error` 换成你的边类型。

## 持有型代理

有时一个对象本身不是边类型 `T`，只是**持有**一个 `T` 字段（例如 `proxyfs` 持有 `Fs` 但没有 `Open` 方法）。这时类型化遍历无法从它起种，用 `AsFrom[E, T]` 显式给出边类型即可：

```go
package main

import (
	"fmt"

	"github.com/corego/typer"
)

type Fs interface{ Open() }
type Fsx interface{ Get() }

type realFs struct{}

func (realFs) Open() {}
func (realFs) Get()  {}

// *proxyfs 并不是 Fs（没有 Open 方法），只是持有 Fs 字段。
type proxyfs struct{ fs Fs }

func (p *proxyfs) Unwrap() Fs { return p.fs }

func main() {
	fsx, ok := typer.AsFrom[Fsx, Fs](&proxyfs{fs: realFs{}})
	if ok {
		fsx.Get() // 命中的是 realFs
	}
}
```

对比两种写法：

- 用 `AsFrom[Fsx, Fs](proxy)`：代理只暴露 `Unwrap() Fs`，自身不必满足 `Fs`。适合「持有但非本身就是」的代理。
- 让 `As[Fsx](proxy)` 直接生效：要求代理自身也实现 `Fs`（即实现 `Open()`），成为透明装饰器，此时它本身就是有效节点，可从它起种。

## 已知边界

- `MaxDepth` 限制的是**深度**而非总工作量。共享子节点的图仍可能代价较高。
- `Is` 使用值相等。非可比较的动态类型不会匹配，但也不会 panic。
- Go 不支持泛型方法：单一类型无法同时实现 `Unwrapper[T]` 与 `MultiUnwrapper[T]`（二者都叫 `Unwrap`）。
- `AsError[error](err)` 对任意非 nil 的 `err` 都会返回 `err` 本身。
- 边类型不做反射发现：只认编译期类型断言，不会运行时扫描字段猜测后继。
- 混合边与 `go vet`：一个同时实现 `error`（有 `Error() string`）与 `Unwrap() T`（`T` 非 `error`）的桥接类型，会被 `go vet` 的 `stdmethods` 检查告警（它假定 `error` 类型的 `Unwrap()` 应返回 `error` 或 `[]error`）。这不是编译错误、语义正确，但 `go vet ./...` 会报警，CI 需要知晓。
