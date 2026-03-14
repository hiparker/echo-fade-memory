#!/usr/bin/env bash
set -e

docker network create echo-fade-memory-net 2>/dev/null || true

echo "==> 启动 Ollama 容器"
if docker ps --format '{{.Names}}' | grep -q '^ollama-nomic-embed-text$'; then
  echo "Ollama 已在运行"
else
  docker run -d --name ollama-nomic-embed-text --network echo-fade-memory-net -p 11434:11434 -v ollama_data:/root/.ollama ollama/ollama 2>/dev/null || \
    (docker start ollama-nomic-embed-text && docker network connect echo-fade-memory-net ollama-nomic-embed-text 2>/dev/null || true)
fi

echo ""
echo "==> 拉取 embedding 模型 (nomic-embed-text)"
docker exec ollama-nomic-embed-text ollama pull nomic-embed-text

echo ""
echo "完成！Ollama embedding 已就绪。"
echo "Ollama 地址: http://ollama-nomic-embed-text:11434"
echo "模型: nomic-embed-text"
