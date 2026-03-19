package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hiparker/echo-fade-memory/pkg/config"
	"github.com/hiparker/echo-fade-memory/pkg/core/engine"
	"github.com/hiparker/echo-fade-memory/pkg/core/model"
	"github.com/hiparker/echo-fade-memory/pkg/portal/api"
	"github.com/hiparker/echo-fade-memory/pkg/portal/web"
)

var errServeHelp = errors.New("serve help requested")

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	command := os.Args[1]

	if command == "serve" {
		if err := applyServeRuntimeOverrides(os.Args[2:]); err != nil {
			if errors.Is(err, errServeHelp) {
				printServeUsage(os.Stdout)
				return
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			printServeUsage(os.Stderr)
			os.Exit(1)
		}
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

	switch command {
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
		fmt.Printf("memory: %s\nsummary: %s\n", res.MemoryID, res.Summary)
		if len(res.SourceRefs) == 0 {
			fmt.Println("sources: none")
			break
		}
		fmt.Printf("sources: %d\nfirst_source: %s\n", len(res.SourceRefs), formatSourceRef(res.SourceRefs[0]))
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
		apiServer := api.NewServer(eng)
		webServer := web.NewHandler()
		mux := http.NewServeMux()
		mux.Handle("/v1/", apiServer)
		mux.Handle("/dashboard", webServer)
		mux.Handle("/dashboard/", webServer)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		})
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("listening on %s\n", addr)
		fmt.Printf("runtime home: %s\n", config.RuntimeHome())
		fmt.Printf("workspace id: %s\n", config.WorkspaceID())
		fmt.Printf("data path: %s\n", cfg.DataPath)
		server := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	writeUsage(os.Stdout)
}

func writeUsage(w io.Writer) {
	fmt.Fprint(w, `echo-fade-memory - a storage system that forgets

Usage:
  echo-fade-memory remember <content> Store a memory
  echo-fade-memory recall <query>     Recall memories by query
  echo-fade-memory reinforce <id>     Reinforce a memory
  echo-fade-memory ground <id>        Show sources for a memory
  echo-fade-memory forget <id>        Delete a memory
  echo-fade-memory decay              Recompute decay for all memories
  echo-fade-memory serve [options]    Start HTTP API server

Serve options:
  --workdir <path>   Runtime home path (same as ECHO_FADE_MEMORY_HOME)
  --workspace <id>   Workspace id override (same as ECHO_FADE_MEMORY_WORKSPACE)
  --port <number>    HTTP port override (same as PORT)
  --help             Show serve options

Environment:
  DATA_PATH   Data directory (default: ~/.echo-fade-memory/workspaces/<workspace>/data)
  ECHO_FADE_MEMORY_HOME  Runtime root (default: ~/.echo-fade-memory)
  ECHO_FADE_MEMORY_WORKSPACE  Override workspace id for data isolation
  EMBEDDING_URL  Embedding API URL for ollama (default: http://localhost:11434)
  CONFIG_PATH Config file (default: config.json)
`)
}

func formatSourceRef(ref model.SourceRef) string {
	if ref.Kind == "" {
		return ref.Ref
	}
	return ref.Kind + ":" + ref.Ref
}

func printServeUsage(w io.Writer) {
	fmt.Fprint(w, `Usage:
  echo-fade-memory serve [--workdir <path>] [--workspace <id>] [--port <number>]
`)
}

func applyServeRuntimeOverrides(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			return errServeHelp
		case arg == "--workdir" || arg == "--home":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for %s", arg)
			}
			i++
			value := strings.TrimSpace(args[i])
			if value == "" {
				return fmt.Errorf("empty value for %s", arg)
			}
			if err := os.Setenv("ECHO_FADE_MEMORY_HOME", value); err != nil {
				return fmt.Errorf("set ECHO_FADE_MEMORY_HOME: %w", err)
			}
		case strings.HasPrefix(arg, "--workdir="), strings.HasPrefix(arg, "--home="):
			parts := strings.SplitN(arg, "=", 2)
			value := strings.TrimSpace(parts[1])
			if value == "" {
				return fmt.Errorf("empty value for %s", parts[0])
			}
			if err := os.Setenv("ECHO_FADE_MEMORY_HOME", value); err != nil {
				return fmt.Errorf("set ECHO_FADE_MEMORY_HOME: %w", err)
			}
		case arg == "--workspace":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --workspace")
			}
			i++
			value := strings.TrimSpace(args[i])
			if value == "" {
				return fmt.Errorf("empty value for --workspace")
			}
			if err := os.Setenv("ECHO_FADE_MEMORY_WORKSPACE", value); err != nil {
				return fmt.Errorf("set ECHO_FADE_MEMORY_WORKSPACE: %w", err)
			}
		case arg == "--port":
			if i+1 >= len(args) {
				return fmt.Errorf("missing value for --port")
			}
			i++
			value := strings.TrimSpace(args[i])
			if value == "" {
				return fmt.Errorf("empty value for --port")
			}
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid value for --port: %q", value)
			}
			if err := os.Setenv("PORT", strconv.Itoa(n)); err != nil {
				return fmt.Errorf("set PORT: %w", err)
			}
		case strings.HasPrefix(arg, "--workspace="):
			parts := strings.SplitN(arg, "=", 2)
			value := strings.TrimSpace(parts[1])
			if value == "" {
				return fmt.Errorf("empty value for --workspace")
			}
			if err := os.Setenv("ECHO_FADE_MEMORY_WORKSPACE", value); err != nil {
				return fmt.Errorf("set ECHO_FADE_MEMORY_WORKSPACE: %w", err)
			}
		case strings.HasPrefix(arg, "--port="):
			parts := strings.SplitN(arg, "=", 2)
			value := strings.TrimSpace(parts[1])
			if value == "" {
				return fmt.Errorf("empty value for --port")
			}
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid value for --port: %q", value)
			}
			if err := os.Setenv("PORT", strconv.Itoa(n)); err != nil {
				return fmt.Errorf("set PORT: %w", err)
			}
		case strings.HasPrefix(arg, "--"):
			return fmt.Errorf("unknown serve option: %s", arg)
		default:
			// Ignore positional args so existing usage remains backward-compatible.
		}
	}
	return nil
}
