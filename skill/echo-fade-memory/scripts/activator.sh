#!/bin/sh
set -eu

cat <<'EOF'
<echo-fade-memory-reminder>
Before answering, check whether durable memory should be part of the workflow:
- Recall relevant prior context first
- Store new durable preferences, decisions, corrections, or lessons
- Reinforce memories that proved useful
- Ground fuzzy memories before trusting them

Useful commands:
- ./scripts/recall-memory.sh "<query>"
- ./scripts/store-memory.sh "<content>" --type preference|project|goal
</echo-fade-memory-reminder>
EOF
