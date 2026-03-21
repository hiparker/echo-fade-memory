# 实现进度：PLAN.zh.md 对照

## 总览

| Phase | 规划内容 | 状态 | 说明 |
|-------|----------|------|------|
| Phase 1 | 核心引擎 | 已完成 | 核心 memory lifecycle、CLI、`/v1/memories`、默认测试矩阵已落地 |
| Phase 2 | 知识图谱 + 多路召回 + Agent/ Dashboard 集成 | Core 已落地 | KG/entity、image recall、`/v1/tools/*`、`/v1/dashboard/*`、Workbench 已可用；relation expansion recall、完整插件化、LLM 级抽取仍在后续 |
| Phase 3 | 人格涌现 | 0% | 未开始；已保留异步 LLM 衰减压缩规划尾巴 |

---

## 1. 五层架构

| 层 | 规划 | 实现 | 说明 |
|----|------|------|------|
| **写入层** | 原始记忆保存 | ✅ | SQLite/Postgres/MySQL 元数据 + 向量 + Bleve + image/KG side stores |
| **时间层** | 衰减调度 | ⚠️ | 惰性：recall 时 DecayAll，无定时 goroutine |
| **变形层** | 摘要→关键词→残影 | ✅ | 已接入 stage-based residual：summary / keywords / fragment；当前仍是规则式 residual |
| **检索层** | 原文/摘要/残影/联想召回 | ✅ | 向量 + BM25 + KG RRF 融合；tool/workbench 还会聚合 image/entity recall |
| **展示层** | CLI 先行 | ✅ | CLI + HTTP API + dashboard/workbench 均已可用 |

---

## 2. 存储模型

| 规划 | 实现 | 说明 |
|------|------|------|
| Memory 单元 | ✅ | 已扩展到 summary, memory_type, lifecycle_state, source_refs, conflict_group, version |
| linkage | ❌ | 无记忆图边，无 linkage 存储 |
| 来源引用 | ✅ | source_refs 已落库，可用于 ground |
| 生命周期状态 | ✅ | fresh / reinforced / weakening / blurred / archived / forgotten |
| 轻量版本化 | ✅ | conflict_group + version 已支持，自动合并/裁决仍未做 |
| 图结构 | ⚠️ | 无完整 memory linkage graph / 簇分析 |
| 知识图谱 | ✅ | SQLite KG、实体/关系/记忆链接已落地，作为 Phase 2 core 能力参与 recall 与 dashboard |

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
| 抽象化有方向 | ⚠️ | 已切到摘要/关键词/片段阶段，但仍是规则式而非 LLM 级语义抽取 |

---

## 4. 召回机制

| 规划 | 实现 | 说明 |
|------|------|------|
| 向量检索 | ✅ | 默认 local JSON；已接入 chromem-go（纯 Go 嵌入式向量库）/ Milvus 后端 |
| BM25 关键词 | ✅ | Bleve |
| 知识图谱 | ✅ | 已作为第三路召回参与 RRF；tool/workbench 还能直接返回 entities |
| RRF 融合 | ✅ | 向量 + BM25 融合 |
| clarity 过滤 | ✅ | minClarity 参数 |
| Explainable Recall | ✅ | 返回 score、strength、freshness、fuzziness、source_refs、why_recalled、needs_grounding |
| 回源 | ✅ | `ground(memory_id)` 已支持来源回查元信息 |
| 联想召回 | ⚠️ | 无「clarity 极低仅向量」逻辑，当前仍是统一融合召回 |
| 图片召回 | ✅ | image vector + keyword + linked-memory boost，tool/workbench 已接入 |
| 关系扩展召回 | ❌ | relation 已存可查，但 recall 还未做 relation walk / expansion |

---

## 5. 技术选型

| 规划 | 实现 | 说明 |
|------|------|------|
| Go | ✅ | |
| chromem-go | ✅ | 纯 Go 嵌入式向量库，替代 LanceDB，零 CGO |
| Local vector store | ✅ | 默认后端，纯 Go，本地开发与测试基线 |
| Milvus | ✅ | 外部服务化后端 |
| Bleve | ✅ | |
| Ollama nomic-embed-text | ✅ | |
| SQLite | ✅ | modernc.org/sqlite 纯 Go |
| Dashboard UI | ✅ | 内嵌静态页面，Overview / Detail / Workbench |

---

## 6. 部署与集成

| 规划 | 实现 | 说明 |
|------|------|------|
| 直接运行 | ✅ | 单二进制 |
| Docker | ✅ | Dockerfile + docker-compose，纯 Go 构建，秒级镜像 |
| 存储可移植 | ✅ | DATA_PATH |
| CLI | ✅ | remember, recall, reinforce, ground, forget, decay |
| HTTP API | ✅ | 已同时提供 core `/v1/memories`、agent `/v1/tools/*`、dashboard `/v1/dashboard/*` |
| MCP Server | ❌ | 未实现 |
| Skill 模板 | ✅ | `skill/echo-fade-memory` 已提供 unified `store/recall/forget` 脚本与文档 |
| 跨平台 | ✅ | Makefile build-all |
| Homebrew/Scoop | ❌ | 未接入 |

---

## 7. Phase 2 / Phase 3 未实现

- OpenClaw 插件
- 情感加权 pipeline（检测「记住这个」等）
- 记忆图 linkage
- 人格轮廓导出
- 异步 LLM 衰减压缩（高质量 residual 改写）
- 联想召回（clarity 极低时仅向量）
- relation expansion recall（按关系扩展记忆召回）
- recall 参数化过滤（如 memory_type / time / relation scope）
- 自动冲突合并 / 事实裁决
- 召回污染检测与审计视图

---

## 8. 与规划的差异

1. **向量存储**：从最初规划的 LanceDB 调整为三后端策略：默认 `local`，`chromem`（纯 Go 嵌入式），以及外部 `milvus`
2. **衰减公式**：规划多因子（linkage、孤立惩罚），实现为简化 strength 公式
3. **Residual**：规划摘要/关键词/情感；当前已实现规则式摘要/关键词/片段，尚非结构化情感抽取。异步 LLM 衰减压缩留待 Phase 3
4. **图能力**：当前优先落地的是 KG/entity graph，而不是完整 memory linkage graph；前者已参与 recall 与 dashboard，后者仍未实现
5. **冲突处理**：当前已有 `conflict_group` + `version` 与 recall 优先最新版本，但尚未自动 merge / arbitrate
6. **Phase 2 完成度**：更适合定义为 “Phase 2 core 已可发布”，而非“Phase 2 愿景全量完成”
