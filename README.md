# Echo Fade Memory

A storage system designed to **forget**. Data fades, distorts, and disappears over time—simulating human memory. Against the era of perfect digital memory.

**定位**：记忆中间件 —— 不是完整 Agent 框架，而是「任何 Agent 都能接」的记忆层。核心能力：写入对话/文档/操作轨迹 → 自动摘要、遗忘、冲突合并 → recall API，支持时间衰减与人格偏置。详见 [docs/CORE.md](docs/CORE.md)。

---

## Documentation

| Language | Plan / 规划 |
| -------- | ----------- |
| [English](docs/PLAN.en.md) | Architecture, roadmap, tech stack |
| [中文](docs/PLAN.zh.md) | 架构设计、实现路径、技术选型 |

---

## Overview

- **Forgetting as a feature**: Selective memory—noise sinks, important things sediment. Personality emerges from what we remember *and* what we forget.
- **AI memory dilemma**: Current AI either remembers everything (context explosion) or nothing (reset). This project fills the gap with a continuum: clear ↔ fuzzy ↔ outline.
- **Three-way recall**: Vector (semantic) + BM25 (keyword) + Knowledge Graph (entity). RRF fusion, clarity filtering.

---

## 概述

- **遗忘即特性**：选择性记忆——噪音下沉，重要沉淀。人格从记住与遗忘中涌现。
- **AI 记忆困境**：现有 AI 要么全记住（上下文爆炸），要么无记忆（每次重置）。本项目填补空白：清晰 ↔ 模糊 ↔ 轮廓。
- **三路召回**：向量（语义）+ BM25（关键词）+ 知识图谱（实体）。RRF 融合，clarity 过滤。

---

## Quick Start

**Prerequisites**: [Go 1.26+](https://go.dev/dl/), [Ollama](https://ollama.ai/) with `nomic-embed-text` model.

```bash
# Pull embedding model
ollama pull nomic-embed-text

# Build
make build

# Store a memory
./echo-fade-memory store "Project meeting: decided to use Go and Bleve for Phase 1"

# Recall
./echo-fade-memory recall "meeting decision"

# HTTP API
./echo-fade-memory serve
# POST /memories {"content":"..."}
# GET /memories?q=query
```

**Docker** (Ollama on host):

```bash
docker compose up --build
# OLLAMA_URL=http://host.docker.internal:11434
```

---

## Configuration

Copy `config.example.json` to `config.json` and customize:

| Section | Key | Description |
|---------|-----|-------------|
| ollama | url, model, dimensions | Ollama embedding API |
| decay | tau, alpha, epsilon | strength = 1/(1+(t/τ)^α) × reinforce; tau=halflife, alpha=shape |
| vector_store | type, path | `local` (default), `lancedb`, `milvus` |
| storage | type, path | `sqlite` (default), `postgres` |

Env vars override config: `OLLAMA_URL`, `OLLAMA_MODEL`, `DECAY_LAMBDA`, `VECTOR_STORE_TYPE`, etc.

**Priority**: Default < config.json < Env
