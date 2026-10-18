# typer

`github.com/corego/typer` 把标准库的 `errors.Unwrap`、`errors.Is`、`errors.As` 泛化为「边类型可配置」的图遍历原语。一套 API 既能走 `error` 链，也能走任意自定义的链式或树状结构。

- Module: `github.com/corego/typer`
- Package: `typer`
- Go: `1.25.4`
- 依赖：仅标准库

## 关键特性

- 把固定 `error` 的 `Unwrap` / `Is` / `As` 泛化到任意边类型 `T`，用法类似 Rust 的 trait。
- 异构查找：`As` 的边类型 `T` 与匹配类型 `E` 相互独立，能在 `T` 链里找出 `E`。
- 混合边图：`AsWithAny[E, T]`（强类型根）/ `AsFromWithAny[E, T]` / `AsFromWith2[E, A, B]` 支持同一图里混用 `Unwrap() T` 与 `Unwrap() any`（或两种边类型），按节点自动降级。
- 零反射：遍历完全基于编译期类型断言，`reflect` 只在 `Is` 里用于防住非可比较动态类型导致的 panic。
- 与标准库语义对齐：typed-nil 节点会被访问并下探，支持 `Unwrap() []error` 树，`errors.Is(err, nil)` 一致，兼容旧的 `interface{ As(any) bool }`。
- 循环安全：`MaxDepth` 深度上限保证自引用或带环图也能终止。

## 安装

```bash
go get github.com/corego/typer
```

## 快速使用

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

	var qerr *QueryError
	if e, ok := typer.As[*QueryError](err); ok {
		qerr = e
	}
	fmt.Println(qerr.Query)                // SELECT
	fmt.Println(typer.Is[error](err, qerr)) // true（同一实例）
}
```

`As[*QueryError](err)` 只显式给出 `E = *QueryError`，边类型 `T` 由实参 `err` 推导为 `error`。

## 典型场景：持有型代理

当代理对象本身不是边类型 `T`，只是持有一个 `T` 字段时，用 `AsFrom` 显式给出边类型：

```go
type Fs interface{ Open() }
type Fsx interface{ Get() }

type realFs struct{}

func (realFs) Open() {}
func (realFs) Get()  {}

// *proxyfs 不是 Fs，只是持有 Fs 字段。
type proxyfs struct{ fs Fs }

func (p *proxyfs) Unwrap() Fs { return p.fs }

fsx, ok := typer.AsFrom[Fsx, Fs](&proxyfs{fs: realFs{}}) // ok == true
```

完整的参数说明与更多示例见 [docs/api.md](./docs/api.md)，设计动机与权衡见 [docs/design.md](./docs/design.md)。

## 与标准库对照（精简）

| 标准库 | 本库 |
| --- | --- |
| `errors.Unwrap(err)` | `Unwrap[error](err)` |
| `errors.Is(err, target)` | `Is[error](err, target)` |
| `errors.As(err, &target)` | `AsError[E](err)` / `As[E, error](err)` |
| `errors.Join(errs...)` | `MultiUnwrapper[error]` |

## 开发与测试

```bash
gofmt -l .
go build ./...
go vet ./...
go test -race ./...
```

本库共有 43 个测试。

许可证：TBD
