# 项目结构

```
echo-fade-memory/
├── cmd/                    # 入口
│   └── echo-fade-memory/   # CLI 主程序
├── docs/                   # 文档
├── pkg/                    # 可复用包
│   ├── config/             # 配置加载
│   ├── basic/              # 基础能力
│   │   └── util/           # 工具
│   │       └── safe/       # 协程安全封装 (Go, Run, Group)
│   ├── core/               # 核心领域
│   │   ├── model/          # 领域模型 (Memory, DecayStage)
│   │   ├── decay/          # 衰减算法
│   │   ├── embedding/      # 嵌入客户端 (Ollama)
│   │   ├── transform/      # 变形层 (Residual)
│   │   └── engine/         # 记忆引擎
│   ├── port/               # 端口/适配器
│   │   └── store/          # 存储接口与实现 (SQLite, Vector, Bleve)
│   └── portal/             # 入口层
│       └── api/            # HTTP API
└── config.example.json
```

## 分层说明

| 层 | 包 | 职责 |
|----|-----|------|
| **cmd** | 入口 | CLI 命令分发 |
| **config** | 配置 | 文件 + 环境变量加载 |
| **basic/util** | 基础工具 | safe.Go, safe.Run, safe.Group |
| **core** | 核心 | 领域模型、衰减、嵌入、变形、引擎 |
| **port/store** | 存储端口 | MemoryStore、VectorStore 接口及实现 |
| **portal/api** | HTTP 入口 | REST API Server |
