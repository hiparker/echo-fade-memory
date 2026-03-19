# OpenClaw 自然触发接入建议

目标：让 agent 在不需要用户显式说“调用 echo-fade-memory”的情况下，自然使用这套长期记忆。

## 触发条件

出现以下语义时，应优先进入 echo-fade-memory 流程：

- 记忆类指令
  - 记住这个
  - 你记一下
  - 以后按这个来
  - 先别忘了
- 跨会话回忆
  - 上次我们说过什么
  - 之前定的是啥
  - 你还记得吗
  - 我不是说过吗
- 稳定信息写入
  - 用户偏好
  - 长期约束
  - 项目决策
  - 用户纠正
  - 可复用 workaround
  - feature request
- 复用记忆回答
  - 需要先回忆上下文再回答的问题

## OpenClaw 内部行为建议

### 回答前

如果问题明显依赖过去信息，先 recall：

```bash
export EFM_BASE_URL=http://host.docker.internal:8080
./skills/echo-fade-memory/scripts/recall-memory.sh "<query>"
```

判断规则：

- `needs_grounding=false`：可直接参考
- `needs_grounding=true`：先 ground 或降级表述为“我查到一条可能相关的记忆”

### 新信息出现时

立即 store，不要等会话结束：

```bash
export EFM_BASE_URL=http://host.docker.internal:8080
./skills/echo-fade-memory/scripts/store-memory.sh \
  "用户偏好极简回答" \
  --type preference \
  --summary "偏好：简洁直接" \
  --importance 0.95 \
  --ref "session:<date>"
```

### 回答确实用上记忆时

立即 reinforce：

```bash
export EFM_BASE_URL=http://host.docker.internal:8080
./skills/echo-fade-memory/scripts/reinforce-memory.sh <memory-id>
```

### 用户要求删除时

```bash
export EFM_BASE_URL=http://host.docker.internal:8080
./skills/echo-fade-memory/scripts/forget-memory.sh <memory-id>
```

## 推荐的 memory_type 映射

| 场景 | memory_type |
|------|-------------|
| 用户偏好 | preference |
| 项目决定 | project |
| 纠正/经验/报错 workaround | project |
| 未来想做的能力 | goal |

## 当前环境注意事项

这个环境里 `127.0.0.1:8080` 不通，应该优先使用：

```bash
export EFM_BASE_URL=http://host.docker.internal:8080
```

## 对 Skill 文案的建议

如果想让 OpenClaw 主模型更稳定触发，建议把下面这段加入 `SKILL.md` 的 Quick Reference 之前：

```md
## Natural Triggers in OpenClaw

Use this skill implicitly when the conversation includes:
- remember this / 记住这个
- what did we decide before / 上次定的是什么
- user preferences, durable constraints, corrections
- project decisions worth carrying across sessions
- repeated failures that reveal a reusable workaround

In OpenClaw, prefer using this workflow proactively instead of waiting for an explicit tool request.
```
