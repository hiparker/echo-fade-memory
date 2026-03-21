# 项目结构

```text
echo-fade-memory/
├── cmd/                    # 入口
│   └── echo-fade-memory/   # CLI 主程序
├── docs/                   # 文档
├── skill/                  # Agent/Skill 集成模板
│   └── echo-fade-memory/   # unified store/recall/forget 脚本与说明
├── images/                 # README / dashboard 截图资源
├── pkg/                    # 可复用包
│   ├── config/             # 配置加载
│   ├── basic/              # 基础能力
│   │   └── util/           # 工具
│   │       └── safe/       # 协程安全封装 (Go, Run, Group)
│   ├── core/               # 核心领域
│   │   ├── model/          # 领域模型 (Memory, Image, Entity, Relation)
│   │   ├── decay/          # 衰减算法
│   │   ├── transform/      # 变形层 (Residual)
│   │   ├── entity/         # 规则式实体抽取 / query terms / graph skeleton
│   │   └── engine/         # 记忆引擎
│   ├── port/               # 端口/适配器
│   │   ├── embedding/      # Embedding provider 接口与实现
│   │   ├── imageproc/      # 图像分析接口与基础实现
│   │   ├── imagestore/     # 图片元数据与 links 存储
│   │   ├── kgstore/        # 实体 / 关系 / memory-entity links 存储
│   │   ├── memstore/       # MemoryStore 接口
│   │   ├── store/          # SQL/Bleve 存储实现
│   │   ├── storefactory/   # 后端装配
│   │   └── vector/         # Vector store 实现 (local/chromem/milvus)
│   ├── static/             # 内嵌 dashboard HTML
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
| **core/model** | 领域模型 | memory、image、entity、relation 等核心对象 |
| **core/decay** | 衰减 | clarity / residual stage 计算 |
| **core/transform** | 变形 | residual 文本抽象与规则式压缩 |
| **core/entity** | 实体层 | 规则式实体抽取、query 分词、graph skeleton |
| **core/engine** | 核心引擎 | store / recall / forget / explain / stats / federated recall |
| **port/memstore** | 记忆元数据端口 | MemoryStore 接口 |
| **port/imagestore** | 图片存储端口 | image asset 与 image links 持久化 |
| **port/kgstore** | KG 存储端口 | entity / relation / memory-entity link 持久化 |
| **port/imageproc** | 图像分析端口 | caption / tags / OCR 输入归一化 |
| **port/store** | 存储实现 | SQL metadata、Bleve index |
| **port/storefactory** | 装配 | 根据配置选择后端 |
| **port/vector** | 向量后端 | local、chromem-go、Milvus |
| **port/embedding** | 嵌入端口 | Provider 接口及实现 (Ollama, OpenAI, Gemini) |
| **test** | 测试 | API、CLI、config、engine 与 testutil |
| **portal/api** | HTTP 入口 | core API、tools API、dashboard API |
| **static** | 展示层资源 | 内嵌 `/dashboard` 页面 |
| **skill** | 集成模板 | agent-facing `store / recall / forget` 脚本、hook、bootstrap 文档 |
