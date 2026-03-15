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
│   │   ├── transform/      # 变形层 (Residual)
│   │   └── engine/         # 记忆引擎
│   ├── port/               # 端口/适配器
│   │   ├── embedding/      # Embedding provider 接口与实现
│   │   ├── memstore/       # MemoryStore 接口
│   │   ├── store/          # SQL/Bleve 存储实现
│   │   ├── storefactory/   # 后端装配
│   │   └── vector/         # Vector store 实现 (local/lancedb/milvus)
│   ├── test/               # 自动化测试 (api/cli/config/engine/testutil)
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
| **core** | 核心 | 领域模型、衰减、变形、引擎 |
| **port/memstore** | 记忆元数据端口 | MemoryStore 接口 |
| **port/store** | 存储实现 | SQL metadata、Bleve index |
| **port/storefactory** | 装配 | 根据配置选择后端 |
| **port/vector** | 向量后端 | local、LanceDB、Milvus |
| **port/embedding** | 嵌入端口 | Provider 接口及实现 (Ollama, OpenAI, Gemini) |
| **test** | 测试 | API、CLI、config、engine 与 testutil |
| **portal/api** | HTTP 入口 | REST API Server |
