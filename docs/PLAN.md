# Echo Fade Memory — Architecture & Roadmap

> A storage system designed to "forget." Data fades, distorts, and disappears over time—simulating human memory.

---

## Core Principle

**[记忆中间件 / Memory Middleware](CORE.md)** — Not a full Agent framework, but a memory layer that any Agent can plug into. Core: store (dialogue, docs, traces) → auto-summary, decay, conflict merge → recall API with time decay and personality bias.

---

## Documentation

| Document | Description |
| -------- | ----------- |
| [**CORE.md**](CORE.md) | 核心开发主旨：记忆中间件定位与能力 |
| [PLAN.en.md](PLAN.en.md) | Architecture & roadmap (English) |
| [PLAN.zh.md](PLAN.zh.md) | 架构设计与实现路径（中文） |

---

## Quick Links

- [**Core principle: Memory Middleware**](CORE.md)
- [Core proposition & personality perspective](PLAN.en.md#core-proposition)
- [Five-layer architecture](PLAN.en.md#engine-first-five-layer-architecture)
- [Storage model & decay algorithm](PLAN.en.md#1-storage-model)
- [Recall mechanism (vector + BM25 + KG)](PLAN.en.md#3-recall-mechanism)
- [Implementation roadmap (Phase 1–3)](PLAN.en.md#4-implementation-roadmap)
- [Deployment & OpenClaw plugin](PLAN.en.md#6-deployment--integration)
