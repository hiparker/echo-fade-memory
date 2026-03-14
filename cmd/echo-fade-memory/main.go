package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

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
		cfg = config.Default()
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
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: echo-fade-memory store <content>")
			os.Exit(1)
		}
		content := os.Args[2]
		m, err := eng.Store(ctx, content, 0.5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("stored: %s\n", m.ID)
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
			fmt.Printf("%d. [%s] clarity=%.2f\n   %s\n\n", i+1, r.Memory.ID[:8], r.Memory.Clarity, r.Memory.ResidualContent)
		}
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
	fmt.Println(`echo-fade-memory - a storage system that forgets

Usage:
  echo-fade-memory store <content>   Store a memory
  echo-fade-memory recall <query>    Recall memories by query
  echo-fade-memory decay            Recompute decay for all memories
  echo-fade-memory serve            Start HTTP API server

Environment:
  DATA_PATH   Data directory (default: ./data)
  OLLAMA_URL  Ollama API URL (default: http://localhost:11434)
  CONFIG_PATH Config file (default: config.json)
`)
}
