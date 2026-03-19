# Echo Fade Memory for OpenClaw

当任务涉及以下内容时，优先使用 echo-fade-memory：
- 跨会话记忆
- 用户偏好、约束、决定
- 用户纠正、错误复盘、可复用经验
- 需要先回忆再回答的问题

服务地址：
- 默认 `http://127.0.0.1:8080`
- 当前环境若在容器内，可设置 `EFM_BASE_URL=http://host.docker.internal:8080`

建议流程：
1. 回答前先 recall
2. 新的 durable 信息立即 store
3. 复用成功后 reinforce
4. 记忆模糊时先 ground

常用命令：
```bash
export EFM_BASE_URL=http://host.docker.internal:8080
./skills/echo-fade-memory/scripts/recall-memory.sh "<query>"
./skills/echo-fade-memory/scripts/store-memory.sh "<content>" --type preference|project|goal
./skills/echo-fade-memory/scripts/reinforce-memory.sh <memory-id>
./skills/echo-fade-memory/scripts/forget-memory.sh <memory-id>
```

触发判断：
- 用户说“记住这个”
- 用户提到“上次”“之前”“我不是说过吗”
- 用户给出明确偏好、长期约束、项目决策
- 某次报错暴露出稳定 workaround
