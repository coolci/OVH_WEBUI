# progress/ — 进度台账

> 唯一事实源：`ledger.csv`（机检）+ Git（权威）。本目录不产生“感觉上的进度”。

| 文件 | 作用 | 谁写 |
|---|---|---|
| `ledger.csv` | 228 条端点的迁移状态（机检） | `tools/status` 从代码重算 |
| `PROGRESS.md` | 断点卡：为什么、下一步做什么 | 每次会话结束由 AI 更新 |

## 状态枚举

| status | 含义 |
|---|---|
| `todo` | 未开始 |
| `wip` | 进行中（不完整，禁止合并） |
| `done` | 满足 DoD（见 07 章 §迁移完成判据），有 commit 证据 |
| `blocked` | 被外部阻塞（须在 PROGRESS.md 写明阻塞项） |

## 为什么台账要能“从代码重算”

如果台账靠人手工标 done，它一定会撒谎 —— 尤其是在 AI 反复中断的场景下。
所以 `tools/status` 的职责是：只读代码与契约，重新推导每一条的状态，然后与台账对账。对不上的时候，以代码为准。

```bash
go run ./tools/status --ledger progress/ledger.csv --verify
```
