# SQLite 深度讨论

## 背景

HighClaw 需要嵌入式数据库支持 TaskLog 存储和 FTS5 全文搜索。

## 选型对比

### CGO SQLite（mattn/go-sqlite3）
- **优点**：原生 C SQLite，性能最优
- **缺点**：需要 CGO，交叉编译困难，增加构建复杂度

### Pure Go SQLite（modernc.org/sqlite）
- **优点**：纯 Go 实现，零 CGO 依赖，完美交叉编译
- **缺点**：性能比原生 C 低 10-30%
- **原理**：C-to-Go 转译，使用 `ccgo` 工具链

### ZeroClaw 的方案
- 使用 `sqlite3` CLI 直接调用
- Rust FFI 绑定 C 库

## 决策

选择 `modernc.org/sqlite`，理由：
1. **纯 Go**：与 HighClaw "纯 Go 工程" 定位一致
2. **性价比**：10-30% 性能损失在嵌入式场景下完全可接受
3. **交叉编译**：无需额外工具链
4. **维护成本低**：标准 Go 构建流程

## FTS5 Trigger 性能

- INSERT 触发器开销：微秒级
- 对比 LLM 调用延迟（秒级）：可忽略不计
- 建议定期 VACUUM/OPTIMIZE 维护索引

## ccgo 工作原理

`ccgo` 是 `modernc.org` 项目的核心工具，将 C 源码逐行翻译为 Go 代码：
1. 解析 C 源码 AST
2. 映射 C 类型到 Go 类型
3. 处理指针运算和内存管理
4. 输出可编译的 Go 代码

## Benchmark 参考

| 操作 | CGO (mattn) | Pure Go (modernc) | 差异 |
|------|-------------|-------------------|------|
| Simple Insert | ~15μs | ~20μs | +33% |
| Simple Select | ~10μs | ~13μs | +30% |
| FTS5 Search | ~50μs | ~65μs | +30% |
| Bulk Insert (1000) | ~5ms | ~6.5ms | +30% |

*数据为近似值，实际取决于硬件和数据规模*
