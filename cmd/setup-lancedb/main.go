// setup-lancedb downloads LanceDB native libraries for the current platform.
// Cross-platform: works on Windows, macOS, and Linux without shell scripts.
//
// Usage:
//
//	go run ./cmd/setup-lancedb
//	go run ./cmd/setup-lancedb --force
//	go run ./cmd/setup-lancedb --static
package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	headerName           = "lancedb.h"
	archiveName          = "lancedb-go-native-binaries.tar.gz"
	defaultGoSourceURL   = "https://github.com/lancedb/lancedb-go.git"
	defaultRustSourceURL = "https://github.com/lancedb/lancedb.git"
)

func main() {
	force := flag.Bool("force", false, "re-download even if files exist")
	static := flag.Bool("static", false, "use static library (.a) instead of dynamic (.dylib/.so)")
	flag.Parse()

	runtimeHome := getRuntimeHome()
	version := getVersion()
	platform, arch := platformArch()
	platformArch := platform + "_" + arch
	releaseBases := getReleaseBaseURLs(version)

	includeDir := filepath.Join(runtimeHome, "include")
	libDir := filepath.Join(runtimeHome, "lib", platformArch)
	headerTarget := filepath.Join(includeDir, headerName)

	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		fatal("mkdir include: %v", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		fatal("mkdir lib: %v", err)
	}

	assetName := ""
	archiveFallback := false
	var libTarget string

	if *static {
		assetName = "liblancedb_go.a"
	} else {
		switch platform {
		case "darwin":
			assetName = "liblancedb_go.dylib"
		case "linux":
			assetName = "liblancedb_go.so"
		case "windows":
			archiveFallback = true
		default:
			fatal("unsupported platform: %s", platform)
		}
	}

	releaseErr := setupFromReleases(releaseBases, headerTarget, libDir, platformArch, assetName, *force, archiveFallback)
	if releaseErr == nil {
		if archiveFallback {
			libTarget = filepath.Join(libDir, "liblancedb_go.a")
			if platform == "windows" {
				fmt.Println("==> note: windows uses static library from release archive")
			}
		} else {
			libTarget = filepath.Join(libDir, assetName)
		}
	} else {
		fmt.Fprintf(os.Stderr, "==> release download failed: %v\n", releaseErr)
		var sourceErr error
		libTarget, sourceErr = buildFromSource(version, platform, arch, headerTarget, libDir, *static)
		if sourceErr != nil {
			fatal("release download failed and source fallback failed: %v", errors.Join(releaseErr, sourceErr))
		}
	}

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

func getReleaseBaseURLs(version string) []string {
	if raw := strings.TrimSpace(os.Getenv("LANCEDB_GO_RELEASE_BASE_URLS")); raw != "" {
		var bases []string
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			bases = append(bases, strings.TrimRight(item, "/"))
		}
		if len(bases) > 0 {
			return bases
		}
	}
	if raw := strings.TrimSpace(os.Getenv("LANCEDB_GO_RELEASE_BASE_URL")); raw != "" {
		return []string{strings.TrimRight(raw, "/")}
	}
	return []string{"https://github.com/lancedb/lancedb-go/releases/download/" + version}
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

func setupFromReleases(baseURLs []string, headerTarget, libDir, platformArch, assetName string, force, archiveFallback bool) error {
	if err := downloadFromBases(baseURLs, headerName, headerTarget, force); err != nil {
		return fmt.Errorf("header: %w", err)
	}
	if !archiveFallback {
		libTarget := filepath.Join(libDir, assetName)
		if err := downloadFromBases(baseURLs, assetName, libTarget, force); err != nil {
			return fmt.Errorf("library: %w", err)
		}
		return nil
	}

	archivePath := filepath.Join(libDir, archiveName)
	if err := downloadFromBases(baseURLs, archiveName, archivePath, force); err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	if err := extractFromArchive(archivePath, "include/"+headerName, headerTarget, force); err != nil {
		return fmt.Errorf("extract header: %w", err)
	}
	libTarget := filepath.Join(libDir, "liblancedb_go.a")
	if err := extractFromArchive(archivePath, "lib/"+platformArch+"/liblancedb_go.a", libTarget, force); err != nil {
		return fmt.Errorf("extract lib: %w", err)
	}
	return nil
}

func downloadFromBases(baseURLs []string, assetName, target string, force bool) error {
	if !force {
		if _, err := os.Stat(target); err == nil {
			fmt.Println("==> reuse", filepath.Base(target))
			return nil
		}
	}

	var errs []error
	for _, baseURL := range baseURLs {
		assetURL := strings.TrimRight(baseURL, "/") + "/" + assetName
		fmt.Println("==> downloading", filepath.Base(target), "from", hostOf(assetURL))
		if err := download(assetURL, target); err != nil {
			errs = append(errs, err)
			continue
		}
		return nil
	}
	if len(errs) == 0 {
		return fmt.Errorf("no release base urls configured for %s", assetName)
	}
	return errors.Join(errs...)
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

func download(url, target string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	tmp := target + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, target)
}

func buildFromSource(version, platform, arch, headerTarget, libDir string, static bool) (string, error) {
	if platform == "windows" {
		return "", fmt.Errorf("source fallback is not yet supported on windows; please provide release assets instead")
	}

	required := []string{"git", "cargo", "rustup", "cbindgen"}
	for _, name := range required {
		if _, err := exec.LookPath(name); err != nil {
			return "", fmt.Errorf("missing %s for source fallback", name)
		}
	}

	workDir, err := os.MkdirTemp("", "setup-lancedb-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workDir)

	sourceDir := filepath.Join(workDir, "lancedb-go")
	sourceURL := strings.TrimSpace(os.Getenv("LANCEDB_GO_SOURCE_URL"))
	if sourceURL == "" {
		sourceURL = defaultGoSourceURL
	}

	fmt.Fprintln(os.Stderr, "==> falling back to source build")
	if err := runCommand(
		workDir,
		nil,
		"git",
		"clone",
		"--depth", "1",
		"--branch", version,
		sourceURL,
		sourceDir,
	); err != nil {
		return "", fmt.Errorf("clone lancedb-go source: %w", err)
	}

	rustSourceURL := strings.TrimSpace(os.Getenv("LANCEDB_RUST_SOURCE_URL"))
	if rustSourceURL != "" && rustSourceURL != defaultRustSourceURL {
		if err := rewriteRustSourceURL(filepath.Join(sourceDir, "rust", "Cargo.toml"), rustSourceURL); err != nil {
			return "", err
		}
	}

	buildEnv := []string{"CARGO_NET_GIT_FETCH_WITH_CLI=true"}
	if err := runCommand(sourceDir, buildEnv, "./scripts/build-native.sh", platform, arch); err != nil {
		return "", fmt.Errorf("build native library from source: %w", err)
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
	updated := strings.ReplaceAll(string(data), defaultRustSourceURL, rustSourceURL)
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
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func extractFromArchive(archivePath, member, target string, force bool) error {
	if !force {
		if _, err := os.Stat(target); err == nil {
			fmt.Println("==> reuse", filepath.Base(target))
			return nil
		}
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("member not found: %s", member)
		}
		if err != nil {
			return err
		}
		if h.Name == member {
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "setup-lancedb: "+format+"\n", args...)
	os.Exit(1)
}
