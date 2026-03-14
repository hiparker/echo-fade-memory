# 核心开发主旨

> 本文档定义 Echo Fade Memory 的定位与开发原则，作为所有设计与实现的指导。

---

## 定位：记忆中间件

**不是完整 Agent 框架，而是「任何 Agent 都能接」的基础设施层记忆引擎。**

Echo Fade Memory 作为**记忆中间件**存在：可被任意 Agent、Skill、RAG 系统或对话应用接入，提供统一的记忆生命周期能力，而不绑定特定框架或产品形态。

分层边界应保持清晰：

- `SKILL.md` / rules 属于策略层，负责“什么时候记、记什么、怎么用”
- Agent framework 属于编排层，负责串联 tools / files / agents
- Echo Fade Memory 属于基础设施层，负责“怎么存、怎么衰减、怎么召回、怎么回源、怎么遗忘”

---

## 核心能力

| 能力 | 说明 |
|------|------|
| **写入** | 对话、文档、操作轨迹等任意可文本化的内容 |
| **多形态记忆** | 原文、摘要、embedding、residual、source refs、lifecycle state 并存 |
| **自动摘要 / 残留** | 随时间从完整内容 → 摘要 → 关键词 → 片段的渐进抽象 |
| **遗忘** | 时间衰减 + 访问强化 + 情感加权，选择性保留 |
| **Explainable Recall** | 返回 `score`、`strength`、`freshness`、`fuzziness`、`why_recalled`、`needs_grounding` |
| **回源** | 记忆不够确定时返回 `source_refs`，由上层决定是否回查事实源 |
| **冲突分组 / 版本化** | 同主题记忆先进入 `conflict_group`，用 `version` 保留演化轨迹 |
| **时间衰减** | strength = f(t, access, importance, emotional)，可配置 |
| **人格偏置** | 高 emotional_weight 的记忆衰减更慢，形成人格锚点 |

---

## 设计原则

1. **接口优先**：HTTP API、CLI 先行，便于任何系统集成
2. **无状态服务**：记忆层独立运行，不依赖特定 Agent 运行时
3. **可配置**：衰减参数、存储后端、向量模型均可配置
4. **可移植**：数据集中在 `DATA_PATH`，可备份、迁移、复现
5. **可解释**：召回不应是黑盒，应能解释为何命中、为何建议回源
6. **兼容上层策略**：与 Skill / Agent 编排层协作，而非替代它们

---

## 当前接口方向

当前代码已开始对齐以下中间件语义：

- `remember(event, meta)`
- `recall(query, context)`
- `reinforce(memory_id)`
- `decay()`
- `ground(memory_id)`
- `forget(policy)`
- `explain(recall_result | query)`

---

## 非目标

- 不做完整 Agent 编排或工作流引擎
- 不做 UI 或终端产品
- 不绑定单一 LLM 或嵌入模型（通过 Ollama 等可插拔）

---

*此主旨贯穿 [PLAN.zh.md](PLAN.zh.md) 的架构设计与实现路径。*
