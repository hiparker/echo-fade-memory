# 实现进度：PLAN.zh.md 对照

## 总览

| Phase | 规划内容 | 状态 | 说明 |
|-------|----------|------|------|
| Phase 1 | 核心引擎 | 约 85% | 五层基本落地，向量用 local 替代 LanceDB |
| Phase 2 | 知识图谱 + LLM 集成 | 约 20% | HTTP API 已有，KG 未做 |
| Phase 3 | 人格涌现 | 0% | 未开始 |

---

## 1. 五层架构

| 层 | 规划 | 实现 | 说明 |
|----|------|------|------|
| **写入层** | 原始记忆保存 | ✅ | SQLite 元数据 + 向量 + Bleve |
| **时间层** | 衰减调度 | ⚠️ | 惰性：recall 时 DecayAll，无定时 goroutine |
| **变形层** | 摘要→关键词→残影 | ✅ | 连续 strength 截断，非离散阶段 |
| **检索层** | 原文/摘要/残影/联想召回 | ✅ | 向量 + BM25 RRF 融合 |
| **展示层** | CLI 先行 | ✅ | store/recall/decay/serve |

---

## 2. 存储模型

| 规划 | 实现 | 说明 |
|------|------|------|
| Memory 单元 | ✅ | id, content, embedding, created_at, last_accessed_at, access_count, importance, emotional_weight, clarity, residual_form, residual_content |
| linkage | ❌ | 无记忆图边，无 linkage 存储 |
| 图结构 | ❌ | 无记忆图，无簇分析 |
| 知识图谱 | ❌ | Phase 2 规划，未实现 |

---

## 3. 衰减算法

| 规划 | 实现 | 说明 |
|------|------|------|
| 时间衰减 | ✅ | strength = 1/(1+(t/τ)^α) |
| 访问强化 | ✅ | reinforce = 1 + ε×(access + importance + emotional) |
| 关联强化 | ❌ | 无 linkage，无法实现 |
| 孤立惩罚 | ❌ | 同上 |
| 情感加权 | ✅ | emotional_weight 参与 reinforce |
| 情感加权 pipeline | ❌ | 无「记住这个」等触发检测 |
| 抽象化有方向 | ⚠️ | 当前为连续截断，非摘要/关键词/情感抽取 |

---

## 4. 召回机制

| 规划 | 实现 | 说明 |
|------|------|------|
| 向量检索 | ✅ | 本地 JSON 向量 + 余弦相似度 |
| BM25 关键词 | ✅ | Bleve |
| 知识图谱 | ❌ | 未实现 |
| RRF 融合 | ✅ | 向量 + BM25 融合 |
| clarity 过滤 | ✅ | minClarity 参数 |
| 联想召回 | ⚠️ | 无「clarity 极低仅向量」逻辑，三路统一召回 |

---

## 5. 技术选型

| 规划 | 实现 | 说明 |
|------|------|------|
| Go | ✅ | |
| LanceDB | ⚠️ | 配置支持，实际用 local JSON（无 CGO） |
| Bleve | ✅ | |
| Ollama nomic-embed-text | ✅ | |
| SQLite | ✅ | modernc.org/sqlite 纯 Go |

---

## 6. 部署与集成

| 规划 | 实现 | 说明 |
|------|------|------|
| 直接运行 | ✅ | 单二进制 |
| Docker | ✅ | Dockerfile + docker-compose |
| 存储可移植 | ✅ | DATA_PATH |
| CLI | ✅ | store, recall, decay |
| HTTP API | ✅ | POST/GET /memories |
| MCP Server | ❌ | 未实现 |
| Skill 模板 | ❌ | 未提供 |
| 跨平台 | ✅ | Makefile build-all |
| Homebrew/Scoop | ❌ | 未接入 |

---

## 7. Phase 2 / Phase 3 未实现

- 知识图谱（实体抽取、第三路召回）
- OpenClaw 插件
- 情感加权 pipeline（检测「记住这个」等）
- 记忆图 linkage
- 人格轮廓导出
- 联想召回（clarity 极低时仅向量）

---

## 8. 与规划的差异

1. **向量存储**：规划 LanceDB，实现为 local JSON（避免 CGO）
2. **衰减公式**：规划多因子（linkage、孤立惩罚），实现为简化 strength 公式
3. **Residual**：规划摘要/关键词/情感，实现为连续截断
4. **记忆图**：规划有向加权图，实现无 linkage
