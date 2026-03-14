# Echo Fade Memory → AI Memory Layer: Architecture Design

> A storage system designed to "forget." Data fades, distorts, and disappears over time—simulating human memory.

## Core Proposition

| Current State              | Target                          |
| ------------------------- | ------------------------------- |
| Context full or reset     | Continuum: clear ↔ fuzzy ↔ gone |
| Undifferentiated storage  | Selective forgetting: noise sinks, important sediments |
| Data accumulation        | Personality formation: association, reinforcement, outline |

### Personality Perspective

Human personality emerges from **selective memory**: we remember values, habits, emotional bonds—not every meal. If AI simulates this:

- **Decay curve**: Information blurs over time; high-importance events decay slower
- **Emotional weighting**: User-expressed preferences, corrections, affirmations get reinforced
- **Personality emergence**: Not preset persona, but "grown" from long-term interaction

| Extreme A                          | Extreme B                    | This Approach                    |
| --------------------------------- | ----------------------------- | -------------------------------- |
| Remember all → noise drowns signal | No memory → stranger every time | Selective retention → personality emerges from memory |

---

## Engine First: Five-Layer Architecture

> Build a time-driven memory degradation engine first, not UI or philosophy. Core capabilities: storage, decay, retrieval, presentation.

| Layer        | Responsibility                                           |
| ------------ | --------------------------------------------------------- |
| **Write**    | How raw memories are saved (format, version, metadata)    |
| **Time**     | How often decay runs, when to trigger transformation      |
| **Transform**| Rules from precise to fuzzy (summary → keywords → residual) |
| **Retrieve** | What is returned: original, summary, residual, or associative recall |
| **Present**  | Visualize "sense of forgetting" (optional; CLI first in Phase 1) |

### Text Degradation Timeline (MVP)

Start with **text** memory degradation, not image blurring—the latter easily drifts into visual effects. Use text to validate whether "forgetting" is a product capability.

| Time    | Form                                      |
| ------- | ----------------------------------------- |
| Day 0   | Full original text                        |
| Day 7   | Compressed to summary                     |
| Day 30  | Keywords + emotion tags only              |
| Day 90  | Fuzzy fragments only                      |
| Day 180 | Not directly accessible; **associative recall** only |

---

## 0. Four Directions to Explore

### 0.1 Essence of Forgetting: Blurring, Not Losing

Human forgetting is not "loss" but **abstraction**. You don't remember what you ate at a dinner five years ago, but you remember whether that period was happy. This shift from detail to outline shapes character—what remains is emotional tone, value judgment, not trivial facts.

**Design implication**: Residual after decay should preserve **emotional tendency, theme, relationships**, not random truncation. Abstraction has direction.

### 0.2 AI Personality Dilemma (Covered)

No middle state → no personality. This project fills that gap.

### 0.3 Product Form: Notes That Age

A "notes that age" app: content fades, blurs, merges over time.

- Older → more abstract (keywords, themes, emotions)
- Recent → clearer (full text, details)
- **The memory decay curve itself is a personality trait**: different users, different parameters, different "memory outlines"

### 0.4 Business Angle: Reverse Selling Point

**"Privacy not recorded"**: Data naturally disappears, no permanent digital trace.

- Target: Users who don't want permanent digital footprints, privacy-sensitive users, B2B "forgettable" scenarios
- Differentiation: Opposite of "permanent cloud storage"

---

## 1. Storage Model

### Memory Unit

```
Memory {
  id: UUID
  content: Embedding + raw text/structured data
  created_at: Timestamp
  last_accessed_at: Timestamp
  access_count: int
  importance: float (0-1, explicit or derived from behavior)
  emotional_weight: float (0-1)  // User-expressed preferences/corrections/affirmations
  linkage: [MemoryRef]  // Semantic/reference links to other memories
  decay_state: {
    clarity: float,      // Current clarity 0-1
    residual: "outline"  // Residual form after full decay
  }
}
```

### Graph Structure: Memory Graph vs Knowledge Graph

**Memory Graph**—current design:

- Nodes = memory units
- Edges = reference / co-occurrence / semantic similarity
- Edge weight = association strength (strengthens with co-activation)

**Knowledge Graph**—optional enhancement:

- Nodes = entities (person, place, concept, event)
- Edges = typed relations (prefers, knows, occurred_at, involves)
- Source = extracted from memory residual, or explicit annotation

| Dimension | Memory Graph      | Knowledge Graph                    |
| --------- | ----------------- | ---------------------------------- |
| Granularity | Memory chunks     | Entities + relations               |
| Edge type | Similarity/reference | Prefers, knows, occurred_at…       |
| Use       | Association reinforcement, cluster analysis | Personality anchors, **one of three recall paths** |
| When      | Phase 1 required  | Phase 2, alongside vector and BM25 |

**Implementation rhythm**: Phase 1 = memory graph + vector + BM25 dual-path recall. Phase 2 = add knowledge graph as third recall path:
- Personality export: "User prefers X", "User knows Y"
- Entity-level recall: "All memories about person/topic X"
- Extract entities from residual into KG as decayed "skeleton"

---

## 2. Decay Algorithm

### Multi-Factor Decay Model

**Clarity** `clarity(t)` is determined by four factors:

```
clarity = f(time_decay, access_reinforcement, linkage_strength, emotional_weight)
```

| Factor           | Formula idea                          | Effect                          |
| ---------------- | ------------------------------------- | ------------------------------- |
| **Time decay**   | Exponential `e^(-λt)`, asymptotic to outline | Recent clear, distant skeleton only |
| **Access reinforcement** | Each recall boosts clarity, Hebbian-like | Frequently recalled stays       |
| **Linkage reinforcement** | Connected to high-weight memories → slower decay | Forms "memory clusters," cluster cores stable |
| **Isolation penalty** | No in/out edges → extra decay coefficient | Isolated info sinks naturally   |
| **Emotional weighting** | User-expressed preferences/corrections → lower decay coefficient | Values, habits, emotional bonds sediment |

### Emotional Weighting

User-strongly-expressed content should decay slower—these are often values, preferences, boundaries, the foundation of personality.

**Trigger signals** (combinable):
- Explicit: "Remember this", "This is important", "You're wrong, it should be…", "Don't say X again"
- Implicit: High emotional intensity (anger, strong affirmation), repeated mention, emphasis
- Behavior: Multiple corrections on same topic → boost that memory's emotional_weight

**Effect**: High emotional_weight memories get a "protection coefficient" in the decay formula, forming long-term stable "personality anchors."

### Residual Form: Abstraction, Not Loss

After full decay, not deletion but **abstraction**—keep outline, discard detail. Aligns with human forgetting.

**Text degradation forms** (aligned with MVP timeline):
- Day 7: Summary
- Day 30: Keywords + emotion tags
- Day 90: Fuzzy fragments
- Day 180: Not directly accessible; **associative recall** only (vector/semantic trigger)

**Principle**: Abstraction has direction—what remains shapes personality (values, emotions, relations), not random truncation. Image degradation (edges, color blocks) deferred to Phase 2+.

---

## 3. Recall Mechanism

### Recall Paths: Three Equally Important

| Path            | Use case                                      | Implementation              |
| --------------- | --------------------------------------------- | --------------------------- |
| **Vector**      | Semantic similarity, conceptual association, "vaguely remember" | LanceDB + nomic-embed-text  |
| **BM25 keyword**| Exact term match, proper nouns, code/config   | Bleve full-text index       |
| **Knowledge graph** | Entity-level query, "all about X", "user prefers Y" | Entity + relation index (Phase 2) |

All three complement each other:
- **Vector**: Same meaning, different words; fuzzy association
- **BM25**: Must contain term; exact hit
- **Knowledge graph**: Structured relations; personality anchors; entity aggregation

**Fusion strategy**: Each path returns top-k → RRF fusion → filter by clarity. KG joins in Phase 2.

**Associative recall**: Memories with very low clarity (e.g., 180 days) cannot be directly key-searched or BM25-hit; only triggered by **vector/semantic association**—simulating "I think I remember", "a vague impression."

### Recall Flow (Three-Way Fusion)

```
recall(query) =
  merge(
    vector_search(query, lance_db),    // Semantic recall
    bm25_search(query, bleve_idx),     // Keyword recall
    kg_search(query, entity_graph)     // KG recall (Phase 2; Phase 1 returns empty)
  ).rrf_fusion()
  .filter(m => m.clarity > recall_threshold)
  .rank_by(relevance × clarity)
  .annotate_with(confidence = f(clarity, relevance))
```

---

## 4. Implementation Roadmap

### Product Form Options

| Form              | Description                          | User value                    |
| ----------------- | ------------------------------------ | ----------------------------- |
| AI memory layer   | Intelligent memory backend for LLM   | Personality emergence, long dialogue |
| Notes that age    | Standalone notes app, content fades   | Decay curve as personality, anti-perfect-memory |
| Privacy-first storage | Data naturally disappears        | Privacy not recorded, no permanent trace |

Start with core engine, then split by scenario.

### Phase 1: Core Engine (Go)

**Goal**: Usable CLI; validate that "forgetting" is a product capability, not an art effect.

- **Go** implementation, single-binary deployment, **CLI first** (presentation layer can add UI later)
- **Deployment-friendly**: `DATA_PATH` configurable, storage centralized; Dockerfile + docker-compose from Phase 1
- **Cross-platform**: Go cross-compilation for macOS (Intel/Apple Silicon), Windows, Linux; GitHub Releases multi-arch binaries; Homebrew/Scoop etc. later
- Five layers: Write (storage format) → Time (decay schedule) → Transform (summary/keywords/residual rules) → Retrieve (three-way recall) → Present (CLI output)
- **MVP text only**: No image blurring; text degradation first (0/7/30/90/180 day timeline)
- Memory unit + **LanceDB** + **Bleve** + Ollama embedding
- Decay algorithm: Align with timeline; configurable λ, reinforcement coefficients
- Recall: Vector + BM25; when clarity very low (e.g., 180 days) **associative recall** only, no direct key lookup

### Phase 2: Knowledge Graph + LLM Integration

- **Knowledge graph**: Entity extraction, relation modeling, third recall path in RRF fusion
- **HTTP API**: `serve` mode exposes REST for skill/agent remote calls
- **Skill template**: Provide `echo-fade-memory` skill example, trigger scenarios and invocation
- **OpenClaw plugin**: TypeScript package, register `store_memory`/`recall_memory` etc. as Agent Tools, call core via HTTP; `openclaw plugins install`
- Memory layer as RAG "long-term memory" backend
- Context window = recent clear memories + fuzzy outlines (expand on demand)
- Recall during dialogue triggers reinforcement

### Phase 3: Personality Emergence

- Analyze memory graph: which clusters stable (high emotional_weight + linkage), which dissipated
- Export "memory outline" as personality prompt—**not preset persona, but grown from long-term interaction**
- Experiment: same base model + different memory history → different "personalities"
- Emotional weighting pipeline: detect strong user expression → boost corresponding memory emotional_weight
- Knowledge graph supports personality description: "User prefers X", "Knows Y" etc.

---

## 5. Technology Choices

### Language: Go

| Consideration | Notes                                                                 |
| ------------- | -------------------------------------------------------------------- |
| Performance   | Strong for storage-intensive apps; GC friendly for this scenario     |
| Dev efficiency| Faster iteration than Rust/C++; open source needs quick validation  |
| Deployment    | Single binary, no runtime dependency; run out of the box              |
| Ecosystem     | LanceDB has [lancedb-go](https://pkg.go.dev/github.com/lancedb/lancedb-go) SDK |
| Fit           | Storage + time decay + vector retrieval; Go sufficient               |

> For extreme-performance low-level engine, consider Rust; for product and open source, Go offers best cost-performance.

### Selected

| Component   | Choice                         | Config                    |
| ----------- | ------------------------------ | ------------------------- |
| Vector store| **LanceDB**                    | Local, lightweight, serverless |
| BM25/Full-text | **Bleve**                    | BM25 scoring, RRF fusion, multilingual tokenization |
| Embedding   | **Ollama + nomic-embed-text**  | Local inference, 768 dim  |

```json
{
  "apiKey": "ollama",
  "model": "nomic-embed-text",
  "baseUrl": "http://host.docker.internal:11434/v1",
  "dimensions": 768
}
```

> `host.docker.internal`: When app runs in Docker, use this to reach host Ollama.

---

## 6. Deployment & Integration

### Deployment Forms

| Form           | Description                                              |
| -------------- | -------------------------------------------------------- |
| **Direct run** | Single binary, `./echo-fade-memory serve`, no runtime dep |
| **Docker**     | Dockerfile + docker-compose, volume mount                 |
| **Portable storage** | All data in `$DATA_PATH` (default `./data`), backup/migrate as a whole |

**Storage layout** (easy backup):

```
data/
├── lancedb/      # Vectors
├── bleve/        # Full-text index
├── memories.db   # SQLite memory metadata
└── config.json   # Optional override
```

Backup: `tar -czvf backup.tar.gz data/`; migrate: extract to new environment.

### Skill-Trigger Friendly

| Method        | Use case                                                          |
| ------------- | ----------------------------------------------------------------- |
| **CLI**       | Skill `exec`: `echo-fade-memory store "..."`, `recall "query"`   |
| **HTTP API**  | REST: `POST /memories`, `GET /memories?q=...`; skill/agent remote |
| **MCP Server**| Optional; Cursor/IDE MCP tool; AI can invoke directly            |
| **Skill template** | Provide `echo-fade-memory.skill` example, when to trigger, how to call |

**Design principle**: Simple interface, stateless, configurable. Docker env: `EMBEDDING_URL`, `DATA_PATH`, `PORT`.

### Cross-Platform Client Install

| Platform   | Install methods                                              |
| ---------- | ------------------------------------------------------------- |
| **macOS**   | Homebrew (`brew install echo-fade-memory`), direct binary    |
| **Windows** | Scoop, Chocolatey, winget, direct `.exe`                     |
| **Linux**   | Direct binary, distro package managers (later)                |

**Go cross-compilation**: Single codebase, `GOOS=darwin/windows GOARCH=amd64/arm64` for multi-platform binaries. Apple Silicon (arm64) and Intel (amd64) supported.

**Release**: GitHub Releases with `echo-fade-memory-darwin-arm64`, `echo-fade-memory-darwin-amd64`, `echo-fade-memory-windows-amd64.exe` etc.; Homebrew tap, Scoop bucket later.

### OpenClaw Plugin

As an **OpenClaw** plugin, expose memory capabilities to AI agents:

| Component | Description                                                       |
| --------- | ---------------------------------------------------------------- |
| **Core**  | Go service, HTTP API, runs standalone                            |
| **Plugin**| TypeScript/JS package, `api.registerTool()` for Agent Tools      |
| **Tools** | `store_memory`, `recall_memory`, `list_memories` etc., LLM-callable |
| **Install** | `openclaw plugins install @scope/echo-fade-memory`            |
| **Config** | Plugin env/config: `ECHO_FADE_MEMORY_URL` (core API address)   |

Plugin is opt-in (`optional: true`); user must enable in `agents.list[].tools.allow`. Core can run locally or in Docker; plugin calls via HTTP.

### TBD

| Component   | Options                                                |
| ----------- | ------------------------------------------------------ |
| Graph/KG    | SQLite (Phase 1 memory graph), Phase 2 KG (Neo4j / custom) |
| Decay schedule | goroutine timer / lazy compute on recall            |

---

## 7. Comparison with Existing Approaches

| Approach        | Memory form        | Forgetting      | Personality      |
| --------------- | ------------------ | --------------- | ----------------- |
| Pure context    | All clear, capped   | Overflow = drop | None              |
| MemGPT / Vector DB | All clear, uncapped | Manual delete   | Preset or none    |
| **This approach** | Continuum: clear→fuzzy→outline | Auto, explainable, reinforceable | Emerges from memory |

---

## Next Steps to Explore

- **Storage model**: Concrete schema, abstraction pipeline (how to extract emotion/theme from detail)
- **Decay formula**: Mathematical derivation, parameter configurability
- **Product form**: "Notes that age" MVP design
- **Business**: Privacy selling point concrete scenarios
- **Integration**: LangChain/LlamaIndex interface design
