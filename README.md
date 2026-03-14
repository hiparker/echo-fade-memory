# Echo Fade Memory

A storage system designed to **forget**. Data fades, distorts, and disappears over time—simulating human memory. Against the era of perfect digital memory.

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
