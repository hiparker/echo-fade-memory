// setup-lancedb builds LanceDB native libraries from source for the current platform.
// Default path is GitHub first, then optional Gitee mirror fallback.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	headerName                 = "lancedb.h"
	defaultGoSourceURL         = "https://github.com/lancedb/lancedb-go.git"
	defaultRustSourceURL       = "https://github.com/lancedb/lancedb.git"
	defaultMirrorGoSourceURL   = "https://gitee.com/hiparker/lancedb-go.git"
	defaultMirrorRustSourceURL = "https://gitee.com/mirrors/lancedb.git"
	upstreamRustSourceURL      = "https://github.com/lancedb/lancedb.git"
)

type sourceCandidate struct {
	goURL   string
	rustURL string
	label   string
}

func main() {
	force := flag.Bool("force", false, "rebuild even if files exist")
	static := flag.Bool("static", false, "use static library (.a) instead of dynamic (.dylib/.so)")
	flag.Parse()

	runtimeHome := getRuntimeHome()
	version := getVersion()
	platform, arch := platformArch()
	platformArch := platform + "_" + arch

	includeDir := filepath.Join(runtimeHome, "include")
	libDir := filepath.Join(runtimeHome, "lib", platformArch)
	headerTarget := filepath.Join(includeDir, headerName)
	requestedLib := filepath.Join(libDir, preferredLibName(platform, *static))

	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		fatal("mkdir include: %v", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		fatal("mkdir lib: %v", err)
	}

	if !*force {
		if _, err := os.Stat(headerTarget); err == nil {
			if _, err := os.Stat(requestedLib); err == nil {
				fmt.Println("==> reuse", filepath.Base(headerTarget))
				fmt.Println("==> reuse", filepath.Base(requestedLib))
				printPaths(runtimeHome, includeDir, headerTarget, requestedLib, platform)
				return
			}
		}
	}

	libTarget, err := buildFromSource(version, platform, arch, headerTarget, libDir, *static)
	if err != nil {
		fatal("source build failed: %v", err)
	}
	printPaths(runtimeHome, includeDir, headerTarget, libTarget, platform)
}

func getRuntimeHome() string {
	if v := strings.TrimSpace(os.Getenv("ECHO_FADE_MEMORY_HOME")); v != "" {
		if strings.HasPrefix(v, "~/") {
			home, _ := os.UserHomeDir()
			if home != "" {
				return filepath.Join(home, v[2:])
			}
		}
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".echo-fade-memory"
	}
	return filepath.Join(home, ".echo-fade-memory")
}

func getVersion() string {
	if v := strings.TrimSpace(os.Getenv("LANCEDB_GO_VERSION")); v != "" {
		return v
	}
	return "v0.1.2"
}

func mirrorEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("LANCEDB_ENABLE_SOURCE_MIRROR"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func sourceCandidates() []sourceCandidate {
	goSourceURL := strings.TrimSpace(os.Getenv("LANCEDB_GO_SOURCE_URL"))
	rustSourceURL := strings.TrimSpace(os.Getenv("LANCEDB_RUST_SOURCE_URL"))
	if goSourceURL != "" || rustSourceURL != "" {
		if goSourceURL == "" {
			goSourceURL = defaultGoSourceURL
		}
		if rustSourceURL == "" {
			rustSourceURL = defaultRustSourceURL
		}
		return []sourceCandidate{{
			goURL:   goSourceURL,
			rustURL: rustSourceURL,
			label:   "custom",
		}}
	}

	candidates := []sourceCandidate{{
		goURL:   defaultGoSourceURL,
		rustURL: defaultRustSourceURL,
		label:   "github",
	}}
	if mirrorEnabled() {
		candidates = append(candidates, sourceCandidate{
			goURL:   defaultMirrorGoSourceURL,
			rustURL: defaultMirrorRustSourceURL,
			label:   "gitee mirror",
		})
	}
	return candidates
}

func platformArch() (platform, arch string) {
	platform = runtime.GOOS
	arch = runtime.GOARCH
	switch arch {
	case "amd64", "386":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		fatal("unsupported architecture: %s", arch)
	}
	if platform == "windows" && arch == "386" {
		arch = "amd64"
	}
	switch platform {
	case "darwin", "linux", "windows":
		return platform, arch
	}
	fatal("unsupported platform: %s", platform)
	return "", ""
}

func buildFromSource(version, platform, arch, headerTarget, libDir string, static bool) (string, error) {
	if platform == "windows" {
		return "", fmt.Errorf("source build is not yet supported on windows")
	}

	required := []string{"bash", "git", "cargo", "rustup", "cbindgen"}
	for _, name := range required {
		if _, err := exec.LookPath(name); err != nil {
			return "", fmt.Errorf("missing %s for source build", name)
		}
	}

	workDir, err := os.MkdirTemp("", "setup-lancedb-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)

	var lastErr error
	for _, candidate := range sourceCandidates() {
		sourceDir := filepath.Join(workDir, "lancedb-go")
		_ = os.RemoveAll(sourceDir)

		fmt.Fprintln(os.Stderr, "==> building from source via", candidate.label)
		fmt.Fprintln(os.Stderr, "==> lancedb-go source:", candidate.goURL)
		if err := runCommand(
			workDir,
			nil,
			"git",
			"clone",
			"--depth", "1",
			"--branch", version,
			candidate.goURL,
			sourceDir,
		); err != nil {
			lastErr = fmt.Errorf("clone lancedb-go source: %w", err)
			continue
		}

		if candidate.rustURL != upstreamRustSourceURL {
			if err := rewriteRustSourceURL(filepath.Join(sourceDir, "rust", "Cargo.toml"), candidate.rustURL); err != nil {
				lastErr = err
				continue
			}
		}

		buildEnv := []string{"CARGO_NET_GIT_FETCH_WITH_CLI=true"}
		if err := runCommand(sourceDir, buildEnv, "./scripts/build-native.sh", platform, arch); err != nil {
			lastErr = fmt.Errorf("build native library from source: %w", err)
			continue
		}

		builtHeader := filepath.Join(sourceDir, "include", headerName)
		if err := copyFile(builtHeader, headerTarget); err != nil {
			return "", fmt.Errorf("copy header: %w", err)
		}

		libName := preferredLibName(platform, static)
		builtLib := filepath.Join(sourceDir, "lib", platform+"_"+arch, libName)
		if _, err := os.Stat(builtLib); err != nil && !static {
			libName = "liblancedb_go.a"
			builtLib = filepath.Join(sourceDir, "lib", platform+"_"+arch, libName)
		}

		libTarget := filepath.Join(libDir, libName)
		if err := copyFile(builtLib, libTarget); err != nil {
			return "", fmt.Errorf("copy library: %w", err)
		}
		return libTarget, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no source candidates configured")
	}
	return "", lastErr
}

func preferredLibName(platform string, static bool) string {
	if static {
		return "liblancedb_go.a"
	}
	switch platform {
	case "darwin":
		return "liblancedb_go.dylib"
	case "linux":
		return "liblancedb_go.so"
	default:
		return "liblancedb_go.a"
	}
}

func rewriteRustSourceURL(cargoTomlPath, rustSourceURL string) error {
	data, err := os.ReadFile(cargoTomlPath)
	if err != nil {
		return fmt.Errorf("read rust/Cargo.toml: %w", err)
	}
	updated := strings.ReplaceAll(string(data), upstreamRustSourceURL, rustSourceURL)
	if updated == string(data) {
		return fmt.Errorf("rust source url placeholder not found in %s", cargoTomlPath)
	}
	if err := os.WriteFile(cargoTomlPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write rust/Cargo.toml: %w", err)
	}
	fmt.Fprintf(os.Stderr, "==> using rust source mirror: %s\n", rustSourceURL)
	return nil
}

func runCommand(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func printPaths(runtimeHome, includeDir, headerTarget, libTarget, platform string) {
	extraLDFlags := ""
	switch platform {
	case "darwin":
		extraLDFlags = "-framework Security -framework CoreFoundation"
	case "linux":
		extraLDFlags = "-ldl -lm -lpthread"
	}

	fmt.Println()
	fmt.Println("LanceDB runtime home:", runtimeHome)
	fmt.Println("Header:", headerTarget)
	fmt.Println("Library:", libTarget)
	fmt.Println()
	fmt.Println("CGO_CFLAGS=-I" + includeDir)
	fmt.Println("CGO_LDFLAGS=" + libTarget + " " + extraLDFlags)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "setup-lancedb: "+format+"\n", args...)
	os.Exit(1)
}
