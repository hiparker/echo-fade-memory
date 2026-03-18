---
name: echo-fade-memory
version: 1.1.0
description: "Runs a decay-aware long-term memory workflow on top of the echo-fade-memory service. Use when an agent needs durable cross-session memory, recall before answering, reinforce reused memories, log corrections/errors/feature requests, or wire hooks/scripts into Cursor, Claude Code, Codex, or OpenClaw."
author: hiparker
keywords: [memory, ai-agent, long-term-memory, decay, forgetting, explainable-recall, chromem, bm25, bleve, hooks, openclaw, cursor, codex, claude-code, project-memory, learnings]
metadata:
  requires:
    runtime:
      - "echo-fade-memory server on http://127.0.0.1:8080 or EFM_BASE_URL"
      - "embedding backend: Ollama or OpenAI or Gemini"
---

# Echo Fade Memory

This skill turns `echo-fade-memory` into an **installed agent memory operating layer**.

It is designed to replace the useful parts of the reference skills with a single project-native workflow:

- Replace `elite-longterm-memory`'s vector memory flow with `echo-fade-memory` recall/store/reinforce/ground
- Replace `self-improving-agent`'s `.learnings/` reminders with hooks that tell the agent to persist durable learnings into memory
- Replace ad-hoc memory scripts with skill-local wrappers in `scripts/`

## Quick Reference

| Situation | Action |
|-----------|--------|
| Start of a task or session | Recall relevant context with `./scripts/recall-memory.sh "<query>"` |
| User states preference / decision / correction | Store immediately with `./scripts/store-memory.sh ...` |
| Agent reuses a memory successfully | Reinforce it with `./scripts/reinforce-memory.sh <id>` |
| Memory result looks fuzzy | Ground it with `GET /v1/memories/<id>/ground` |
| Command/tool failure reveals a reusable lesson | Store an error/learning memory with high importance |
| User requests unsupported capability | Store as a feature-request memory |
| Periodic cleanup / freshness | Run `POST /v1/memories/decay` |

## Replacement Mapping

| Reference skill capability | Replacement in this package |
|----------------------------|-----------------------------|
| `elite-longterm-memory` warm vector store | `echo-fade-memory` vector + BM25 + explainable recall |
| `elite-longterm-memory` manual memory commands | `scripts/store-memory.sh` and `scripts/recall-memory.sh` |
| `self-improving-agent` activator hook | `scripts/activator.sh` + `hooks/openclaw/handler.js` |
| `self-improving-agent` error reminder | `scripts/error-detector.sh` |
| `self-improving-agent` examples/references | `references/*.md` and `assets/*.md` |

## Core Workflow

### 1. Recall Before Responding

Before answering about prior decisions, preferences, goals, or unresolved issues:

```bash
./scripts/recall-memory.sh "database choice for this project"
```

Check:

- `why_recalled`
- `needs_grounding`
- `evidence`

If confidence is low, call `/ground` before relying on it.

### 2. Store Durable Facts Early

When the user says something durable, store it **before** moving on:

```bash
./scripts/store-memory.sh \
  "User prefers dark mode and minimal UI" \
  --type preference \
  --summary "dark mode preference" \
  --importance 0.95 \
  --ref "session:2026-03-18"
```

Use high importance for:

- preferences
- corrections
- project decisions
- constraints
- explicit "remember this" statements

### 3. Reinforce Reused Memories

If a memory was recalled and actually helped:

```bash
./scripts/reinforce-memory.sh <memory-id>
```

This increases `access_count`, updates `last_accessed_at`, and slows decay.

### 4. Capture Learnings and Errors as Memories

Instead of `.learnings/ERRORS.md` or `.learnings/FEATURE_REQUESTS.md`, this skill stores durable operational lessons in memory.

Recommended mapping:

| Type of learning | Suggested `memory_type` | Notes |
|------------------|-------------------------|-------|
| User preference | `preference` | Use high importance |
| Project decision | `project` | Add `conflict_group` for versioning |
| Error workaround | `project` | Prefix summary with `error:` or `learning:` |
| Missing feature / future enhancement | `goal` or `project` | Prefix summary with `feature-request:` |

### 5. Decay and Grounding

Memories are not static. They fade over time:

```
full -> summary -> keywords -> fragment -> outline
```

Strength formula:

```text
strength = decay * reinforce
decay     = 1 / (1 + (age_days / tau)^alpha)
reinforce = 1 + epsilon * (access_count + importance + emotional_weight)
```

Current practical note:

- `importance` is writable now and should be used to express salience
- `emotional_weight` is reserved in the model but not yet exposed by the API

## Scripts

| Script | Purpose |
|--------|---------|
| `scripts/health-check.sh` | Verify the server is reachable |
| `scripts/store-memory.sh` | Store preference/decision/error/feature memories |
| `scripts/recall-memory.sh` | Query recall API with optional `k` |
| `scripts/reinforce-memory.sh` | Reinforce a recalled memory |
| `scripts/forget-memory.sh` | Delete a memory |
| `scripts/activator.sh` | Hook reminder for recall/store discipline |
| `scripts/error-detector.sh` | Hook reminder when command output looks like a failure |

## Setup

### Service Availability

```bash
# the service should already be running
./scripts/health-check.sh
```

### Service via Docker

```bash
# if your deployment source includes this repository
docker compose -f docker-compose.ollama.yml up -d
./scripts/health-check.sh
```

The scripts default to `http://127.0.0.1:8080`.

Override with:

```bash
export EFM_BASE_URL=http://host:8080
```

## Agent Rules

### On Session Start

1. Recall project context before deep work.
2. Prefer recalling with concrete queries over broad dumps.
3. If the memory looks vague, ground it before using it.

### During Conversation

1. Durable fact appears -> store it.
2. Durable fact is reused -> reinforce it.
3. User says to forget -> delete it.
4. Command failure teaches a lesson -> store a learning memory.

### On Session End

1. Store any important unresolved blocker or next-step decision.
2. Optionally trigger decay on maintenance windows.

## Additional Resources

- Usage examples: [references/examples.md](references/examples.md)
- Hook setup: [references/hooks-setup.md](references/hooks-setup.md)
- OpenClaw integration: [references/openclaw-integration.md](references/openclaw-integration.md)
- Memory payload templates: [assets/memory-templates.md](assets/memory-templates.md)

## Gotchas

- `needs_grounding: true` means "verify before trusting", not "discard it".
- Use `importance` now for emotional salience until `emotional_weight` is wired through the API.
- This skill replaces the *workflow* of the reference skills, not their exact storage layout.
- The service is the source of truth; file-based memory logs are optional, not required.

## Links

- Repository: https://github.com/hiparker/echo-fade-memory
- Project overview: `README.md`
- Architecture: `docs/PLAN.en.md`, `docs/PLAN.zh.md`
