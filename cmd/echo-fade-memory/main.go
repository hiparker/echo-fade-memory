package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/echo-fade-memory/echo-fade-memory/pkg/config"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/core/engine"
	"github.com/echo-fade-memory/echo-fade-memory/pkg/portal/api"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
	eng, err := engine.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	ctx := context.Background()

	switch os.Args[1] {
	case "store":
		fallthrough
	case "remember":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory remember <content>")
			os.Exit(1)
		}
		content := os.Args[2]
		m, err := eng.Store(ctx, content, 0.5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("stored: %s (%s)\n", m.ID, m.LifecycleState)
	case "recall":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory recall <query>")
			os.Exit(1)
		}
		query := os.Args[2]
		k := 5
		results, err := eng.Recall(ctx, query, k, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for i, r := range results {
			fmt.Printf("%d. [%s] score=%.2f strength=%.2f freshness=%.2f stage=%s\n   %s\n   why=%s\n\n", i+1, r.Memory.ID[:8], r.Score, r.Strength, r.Freshness, r.DecayStage, r.Summary, strings.Join(r.WhyRecalled, ","))
		}
	case "reinforce":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory reinforce <memory_id>")
			os.Exit(1)
		}
		m, err := eng.Reinforce(ctx, os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if m == nil {
			fmt.Fprintln(os.Stderr, "memory not found")
			os.Exit(1)
		}
		fmt.Printf("reinforced: %s (%d accesses)\n", m.ID, m.AccessCount)
	case "ground":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory ground <memory_id>")
			os.Exit(1)
		}
		res, err := eng.Ground(ctx, os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if res == nil {
			fmt.Fprintln(os.Stderr, "memory not found")
			os.Exit(1)
		}
		fmt.Printf("memory: %s\nsummary: %s\nsource: %s\n", res.MemoryID, res.Summary, res.Source)
	case "forget":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory forget <memory_id>")
			os.Exit(1)
		}
		if err := eng.Forget(ctx, os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("forgotten")
	case "decay":
		if err := eng.DecayAll(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("decay applied")
	case "serve":
		port := cfg.Port
		if p := os.Getenv("PORT"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				port = n
			}
		}
		srv := api.NewServer(eng)
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("listening on %s\n", addr)
		if err := http.ListenAndServe(addr, srv); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`echo-fade-memory - a storage system that forgets

Usage:
  echo-fade-memory remember <content> Store a memory
  echo-fade-memory recall <query>     Recall memories by query
  echo-fade-memory reinforce <id>     Reinforce a memory
  echo-fade-memory ground <id>        Show sources for a memory
  echo-fade-memory forget <id>        Delete a memory
  echo-fade-memory decay              Recompute decay for all memories
  echo-fade-memory serve              Start HTTP API server

Environment:
  DATA_PATH   Data directory (default: ~/.echo-fade-memory/workspaces/<workspace>/data)
  ECHO_FADE_MEMORY_HOME  Runtime root (default: ~/.echo-fade-memory)
  ECHO_FADE_MEMORY_WORKSPACE  Override workspace id for data isolation
  EMBEDDING_URL  Embedding API URL for ollama (default: http://localhost:11434)
  CONFIG_PATH Config file (default: config.json)
`)
}
