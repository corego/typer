# 设计文档

本文说明 `github.com/corego/typer` 为什么这样设计。想直接查用法，请看 [api.md](./api.md)；只想快速上手，请看 [../README.md](../README.md)。

## 1. 背景与动机

标准库的 `errors.Unwrap`、`errors.Is`、`errors.As` 非常好用，但它们把链节类型钉死在 `error` 上：所有 `Unwrap()` 都必须返回 `error`。这意味着如果你想遍历一棵节点类型不是 `error` 的图（比如 AST、目录树、表达式链、资源依赖图），就只能自己再写一套。

本库把这件事泛化：把链节类型抽成一个类型参数 `T`（我们叫**边类型**），再提供一组和标准库同名的原语。于是同一套 `Unwrap`、`Walk`、`As`、`Is` 既能跑 `error` 链，也能跑任意自定义的链式结构，用法上贴近 Rust 的 trait 思路：节点通过实现接口来声明自己的后继与匹配行为。

这条路并非没有前车之鉴。Go 提案 #33473（"generalize `As` to all types"，2019）想把 `errors.As` 从 `error` 推广到任意类型，最终被拒，核心理由是语义含糊：一旦放开类型，就说不清「边类型」和「匹配类型」到底是什么关系。本库是对这个遗留问题的一次重新作答。我们给出的答案是：**把两种类型显式分离**，并用「部分类型实参」把常见调用写得足够简洁。

## 2. 核心设计决策

下面每条给出结论和理由。

### 2.1 异构 `As[E, T]`：边类型与匹配类型分离

**结论**：匹配类型 `E` 与边类型 `T` 是两个独立的类型参数，`As[E, T](root T) (E, bool)`。

**理由**：这才是 `errors.As` 的真正泛化。在标准库里，`errors.As(err, &target)` 中「链的边」固定是 `error`，而你捞出来的「匹配目标」是另一个类型，两者天然不同。如果强行让它们同名变成 `As[T](v any)`，会遇到两头堵：

- 当 `T` 是接口（例如 `error`）时，由于 `any(v).(T)` 对根节点几乎总能命中，遍历会在根就停下，根本走不动链。
- 当 `T` 是具体类型时，又会要求整条链的每个节点都是同一个具体类型，现实里链上的节点类型通常是多样的。

把 `T` 和 `E` 分开后，`T` 决定「图长什么样」，`E` 决定「你想从图里捞什么」，两个问题各归各位。这正是提案 #33473 没讲清的地方。

### 2.2 部分类型实参：兼顾简洁与推导

**结论**：`As` 的调用形式是 `As[E](root)`，只显式给出 `E`，`T` 由实参 `root` 推导。

**理由**：Go 支持「部分类型实参」：显式传入的类型实参从左往右匹配类型参数列表，剩下的留给类型推导。`As[E any, T any]` 里，写 `As[*MyErr](err)` 时 `E = *MyErr` 是显式给出的，`T` 则由 `err` 的动态静态类型推导为 `error`。这样既保留了异构的完整表达力，又让绝大多数调用一眼就懂，不必每次都写足两个类型实参。

`AsFrom[E, T](root any)` 是例外：它的 `root` 是 `any`，无法从中推导 `T`，所以两个类型实参都得显式给出，例如 `AsFrom[Fsx, Fs](root)`。

### 2.3 零反射遍历

**结论**：遍历完全用编译期类型断言，形如 `any(v).(Unwrapper[T])`。`reflect` 仅出现在 `Is` 的非可比较防护里。

**理由**：泛型在 Go 里编译期就能把 `Unwrapper[T]`、`MultiUnwrapper[T]`、`Aser[E]` 这些接口断言落实到具体代码，运行时没有反射开销，也不存在「字段扫描猜后继」这种脆弱逻辑。`reflect` 唯一必要的用武之地是 `Is` 的相等比较：`T comparable` 只约束静态类型，一个静态上可比较、但动态值里嵌了 interface 且那个 interface 持有切片的类型，直接用 `==` 会 panic。我们用 `isEqualValue` 里的 `reflect.Value.Comparable()` 在**值级**检查（而不是只看类型级），遇到不可比较的动态值就判为不相等，而不是让程序崩溃。这个值级检查是刻意的：只看静态类型不足以发现「结构体内嵌 interface，运行时塞了切片」这类情况。

### 2.4 `Is` 采用哨兵相等，而非类型成员判定

**结论**：`Is[T](root, target)` 与 `errors.Is` 一样是**哨兵相等**（sentinel identity），不做「这个节点是否属于某类型」的判定。

**理由**：保留 `errors.Is` 的既有语义，用户迁移零心智负担。如果把 `Is` 改成类型成员判定，那它和 `As` 就等价了：两者都能回答「图里有没有某个类型的节点」，合并只会让 API 冗余，还破坏了「`Is` 问相等、`As` 问类型」这个已经被标准库培养出来的直觉。所以 `Is` 忠于平等，类型相关的问题交给 `As`。

### 2.5 覆盖接口：`Iser[T]` 与 `Aser[T]`

**结论**：节点可以可选地实现 `Iser[T]`（`Is(T) bool`）和 `Aser[T]`（`As(*T) bool`）来覆盖默认行为，同时兼容标准库旧式的 `interface{ Is(error) bool }` 与 `interface{ As(any) bool }`。

**理由**：标准库早就允许错误类型自定义 `Is(error) bool` 和 `As(any) bool`，泛化后必须给出对应物。`Iser[T]` 对应 `Is(error) bool`，`Aser[T]` 对应 `As(any) bool`。区别在于 `Aser` 是**类型化**的 `As(*T) bool`，因为泛型里我们能知道匹配类型 `E`，没必要退化到 `any`。同时我们仍保留对旧式 `interface{ As(any) bool }` 的识别，让已经按标准库写法实现的自定义类型无需改动就能被本库认出来。匹配优先级是：直接断言 → `Aser[E]` → 旧式 `As(any) bool`。

### 2.6 循环安全：只用深度上限

**结论**：用常量 `MaxDepth = 1_000` 作为遍历深度的硬上限，保证自引用或成环图能终止。不做 visited-set，也不做 `next == v` 的自环特判。

**理由**：

- 不做 visited-set，是因为 `T` 是任意的，要给任意 `T` 建一个去重集合就得求哈希，而 `T` 未必可比较、更未必可哈希。深度上限对任意 `T` 都成立，代价也恒定。
- 不做 `next == v` 自环防护，是因为它对非可比较类型不安全（`==` 会 panic），而且即便成立也只能挡住一元的自环，挡不住 `A -> B -> A` 这类二元环。与其做一个有漏洞且不安全的特判，不如统一交给深度上限。

超过 `MaxDepth` 的节点被视为不存在：既不访问也不下探。

### 2.7 与标准库对齐

**结论**：typed-nil 节点会被访问并下探；`errors.Is(err, nil)` 语义一致；支持 `Unwrap() []error` 树；兼容旧式 `As(any) bool` 覆盖。

**理由**：这些行为在评审中被反复讨论。最初的一个版本把 typed-nil 节点视为「链的终止点」，看起来更干净，但和标准库的 `errors.Is` / `errors.As` 不一致（标准库会访问 nil 指针包装的节点并继续下探）。为了不让用户在标准库和本库之间产生认知分裂，我们最终决定**与标准库对齐**：一个装进接口的 `(*myErr)(nil)` 仍会被访问，并照常下探。真正的 nil 接口（`any(v) == nil`）才会被跳过。

### 2.8 Go 语言约束带来的后果

**结论**：单一类型无法同时实现 `Unwrapper[T]` 与 `MultiUnwrapper[T]`。

**理由**：两个接口都声明了名为 `Unwrap` 的方法，返回类型不同（`T` 与 `[]T`）。Go 不支持方法重载，也不允许接口方法自带类型参数，所以一个具体类型没法同时满足两者。由此，遍历器里「先试单子节点，再试多子节点」的两个分支实际上永远不会在同一次节点访问中同时命中，这个顺序更多是为了逻辑清晰而非消歧。

### 2.9 混合边：按节点的 `any` 降级与多边类型

**结论**：新增 `AsFromWithAny[E, T]` 与 `AsFromWith2[E, A, B]`，支持边类型不同的节点混在同一个图里。它们共用一个以 `any` 承载节点的遍历器，对**每个节点按序尝试**多个边形状：`AsFromWithAny` 依次试 `Unwrapper[T]`、`MultiUnwrapper[T]`、`Unwrapper[any]`、`MultiUnwrapper[any]`；`AsFromWith2` 多试 `A`、`B` 两种边形状，最后同样降级到 `any`。

**理由**：`walkNodes[T]` 是**同质**遍历——边类型 `T` 的遍历只会跟 `Unwrap() T` 的边。这带来一个盲区：节点实现 `Unwrap() any`（或 `Unwrap() error`、`Unwrap() Fs`）时，对 `T` 边遍历完全不可见，会被当成死胡同。真实代码里混合边并不罕见：`error` 链中某个节点又挂了一条自定义边，或持有型代理混用 `Unwrap() Fs` 与 `Unwrap() any`。

- **降级必须按节点做，不能整链做**。若做成「先跑 `As[E,T]`，没找到再跑 `As[E,any]`」，混合链会两头失败：`T` 遍历在第一个 `any` 节点停住，`any` 遍历在第一个 `T` 节点停住。只有按节点尝试才能跨过形状边界。
- **无歧义**：Go 禁止同名方法重载（§2.8），一个具体类型至多实现一种 `Unwrap` 形状。因此「先 `T` 后 `any`」的顺序在每个节点上至多命中一种，语义天然确定，也不会重复下探。
- **向后兼容**：`any` 降级只增加可达节点（不实现 `T` 形状的节点原本就是死胡同），永远不会减少原有结果。
- **为什么支持到 `any` 就够**：`any` 是通配边，任何节点都能把自己的后继改成 `Unwrap() any` 而不丢类型信息。`AsFromWith2` 覆盖「两种具体边互不可见」（如 `error` 边与 `Fs` 边）的场景，代价是每多一种边类型就要多一个类型参数——Go 没有可变类型参数，只能按组合数提供固定元数版本。
- **与 `AsFrom` 的差异**：混合遍历把根也当作普通节点访问并匹配（与 `As` 一致），因此根实现 `Aser[E]` 或旧式 `As(any) bool` 时也能命中；`AsFrom` 的根只在「直接可赋值给 `E`」时命中。

## 3. 遍历算法

所有查找原语的底座是 `walkNodes`：先序、深度优先，第一个命中即返回（短路）。每个节点上先访问、再尝试下探；下探时优先 `Unwrapper[T]`（单子节点），否则看 `MultiUnwrapper[T]`（多子节点，按序）。终止条件是 `depth > MaxDepth` 或 `any(v) == nil`。

```go
func walkNodes[T any](root T, visit func(T) bool) bool {
	var rec func(v T, depth int) bool
	rec = func(v T, depth int) bool {
		if depth > MaxDepth || any(v) == nil {
			return false
		}
		if visit(v) { // visit 返回 true 表示提前停止
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
```

返回值是「是否被提前停止」：`true` 表示某次 `visit` 返回了 `true`，`false` 表示完整跑完。

`As`、`Is` 都构建在 `walkNodes` 之上：`As` 的 `visit` 里依次做直接断言、`Aser[E]`、旧式 `As(any) bool`；`Is` 的 `visit` 里依次做值相等、`Iser[T]`。命中即让 `visit` 返回 `true` 触发短路。

## 4. 内部模块划分

从实现视角看，代码分两层：

- **`internal/unwrap`**：真正的实现，包含 `walkNodes`、`matchNode`、`isEqualValue` 以及全部核心逻辑。
- **根包 `typer`**：一个薄薄的门面，只做三件事：
  1. **泛型类型别名**：`type Unwrapper[T any] = unwrap.Unwrapper[T]`，其余接口同理。
  2. **常量别名**：`const MaxDepth = unwrap.MaxDepth`。
  3. **薄转发函数**：`Unwrap`、`UnwrapAll`、`Walk`、`As`、`AsFrom`、`Is`、`AsError` 都转发到 `internal/unwrap` 的对应实现。

**为什么这样分**：

- **实现细节不可被外部依赖**。放进 `internal/` 后，包外用户无法 import `internal/unwrap`，我们因此获得修改内部实现的自由，公开面被严格限制在根包。
- **公开面保持干净**。用户只需要记 `github.com/corego/typer` 一个导入路径。
- **导入路径与所有导出名不变**。因为根包用的是**真别名**（`=` 号），不是新定义的命名类型，所以用户自定义类型实现的 `Unwrap() error` 依然满足 `typer.Unwrapper[error]`，别名透明传递，不会有任何「类型身份对不上」的问题。

需要强调：`type Unwrapper[T any] = unwrap.Unwrapper[T]` 里的等号是关键。如果写成不带等号的新类型定义，用户就得重新实现接口，别名意义尽失。

## 5. 权衡与已知边界

- **`MaxDepth` 限的是深度，不是工作量**。它保证终止，但不保证快。一个节点被多个父节点共享的图（DAG）可能被反复下探，最坏情况下访问次数随深度指数增长。这是用「对任意 `T` 都简单安全」换来的代价。
- **非可比较类型不参与 `Is` 相等**。动态值含不可比较成分时，`Is` 判为不相等，但不会 panic。
- **`AsError[error](err)` 对任意非 nil 的 `err` 都返回 `err` 本身**。因为 `E = error` 时，根节点直接满足断言。
- **边类型不做反射发现**。只认编译期接口断言，不会在运行时扫描结构体字段去猜后继节点。后继必须通过 `Unwrapper[T]` 或 `MultiUnwrapper[T]` 显式声明。
- **`error` 类型 + 非 error 的 `Unwrap` 会触发 `go vet` 的 `stdmethods` 告警**。`stdmethods` 假定实现了 `error` 的类型的 `Unwrap()` 应返回 `error` 或 `[]error`，因此 `AsFromWith2[E, error, Fs]` 场景里的桥接类型（既有 `Error() string` 又有 `Unwrap() Fs`）会被 `go vet ./...` 报告。语义正确、不是编译错误，但 CI 需要知晓。

## 6. 可能的后续方向

- **异构边遍历（`AsFromWithAny` / `AsFromWith2`）已实现**，见 §2.9。剩下的组合（三边及以上）可按同样模式继续加固定元数版本。
- **`IsDeep[T]`**：可选地提供基于 `reflect.DeepEqual` 的深度相等判定，作为 `Is` 的补充，应对需要按内容而非哨兵比较的场景。目前没有实现。
- **visited-set 去重**：在受信任、且 `T` 受限可哈希的边界内，引入显式去重来抑制 DAG 的重复访问。代价是丧失对任意 `T` 的通用性，因此只可能作为可选项出现。

以上都只是方向，当前公开 API 里并不存在这些符号。
