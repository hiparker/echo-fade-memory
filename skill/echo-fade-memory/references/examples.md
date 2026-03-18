# Examples

Concrete usage patterns for running agent memory on top of `echo-fade-memory`.

Assumption in this file:

- commands are run from the installed skill directory
- so script paths use `./scripts/...`

## 1. Store a Preference

```bash
./scripts/store-memory.sh \
  "User prefers concise answers and dislikes nested bullets" \
  --type preference \
  --summary "response style preference" \
  --importance 0.95 \
  --ref "session:style"
```

Why:

- durable preference
- should affect future replies
- high importance slows decay

## 2. Store a Project Decision

```bash
./scripts/store-memory.sh \
  "Project decision: use chromem-go as the embedded vector store to keep setup lightweight and dependency-free" \
  --type project \
  --summary "vector backend decision" \
  --importance 0.90 \
  --conflict-group "project:vector-backend" \
  --ref "session:architecture"
```

Use `conflict_group` when the same decision may get revised later. The engine will version the memories.

## 3. Store a Reusable Error Workaround

```bash
./scripts/store-memory.sh \
  "Learning: system git at /usr/bin/git supports --trailer while legacy /usr/local/bin/git 2.23 does not." \
  --type project \
  --summary "error: use system git for trailer support" \
  --importance 0.88 \
  --ref "session:git-fix"
```

This replaces a `.learnings/ERRORS.md` style entry with a searchable memory.

## 4. Store a Feature Request

```bash
./scripts/store-memory.sh \
  "Feature request: expose emotional_weight in POST /v1/memories so agents can mark emotionally salient facts." \
  --type goal \
  --summary "feature-request: emotional weight api" \
  --importance 0.70 \
  --ref "session:feature-request"
```

## 5. Recall Before Answering

```bash
./scripts/recall-memory.sh "vector backend decision"
```

Typical response fields to inspect:

- `id`
- `score`
- `why_recalled`
- `needs_grounding`

If the result has `needs_grounding: true`, verify via the ground endpoint.

## 6. Ground a Fuzzy Memory

```bash
curl -sS "$EFM_BASE_URL/v1/memories/<id>/ground"
```

Useful when:

- a memory is old
- the result is fragmentary
- you need source references before using it in a reply

## 7. Reinforce After Successful Reuse

```bash
./scripts/reinforce-memory.sh <id>
```

Recommended when:

- the user confirms the recalled fact
- the memory directly helped solve the current task

## 8. Forget Incorrect or Obsolete Memory

```bash
./scripts/forget-memory.sh <id>
```

Use when:

- the user explicitly says to forget
- the memory is unsafe or wrong
- the memory was superseded and should not be recalled again

## 9. Explain Recall

```bash
curl -sS -X POST "$EFM_BASE_URL/v1/memories/explain" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "what did we decide about vector storage",
    "k": 5,
    "min_clarity": 0
  }'
```

Use explain mode when you need:

- accepted vs filtered candidates
- confidence debugging
- recall quality inspection

## 10. Suggested Memory Taxonomy

| Situation | `memory_type` | Summary prefix |
|-----------|---------------|----------------|
| User preference | `preference` | `preference:` optional |
| Project decision | `project` | `decision:` optional |
| Goal / pending work | `goal` | `goal:` optional |
| Error workaround | `project` | `error:` or `learning:` |
| Capability request | `goal` | `feature-request:` |

## 11. Prompt-Level Pattern

```text
User: "Remember that we switched to chromem because we wanted the embedded vector store to stay lightweight and easy to run."

Agent (internal):
1. Store memory with importance 0.95
2. Respond to user

Later...

User: "Why did we switch to chromem?"

Agent (internal):
1. Recall memory
2. If needed, ground it
3. Answer
4. Reinforce the memory if it was used successfully
```
