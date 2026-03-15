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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	version     = "v0.1.2"
	baseURL     = "https://github.com/lancedb/lancedb-go/releases/download/" + version
	headerName  = "lancedb.h"
	archiveName = "lancedb-go-native-binaries.tar.gz"
)

func main() {
	force := flag.Bool("force", false, "re-download even if files exist")
	static := flag.Bool("static", false, "use static library (.a) instead of dynamic (.dylib/.so)")
	flag.Parse()

	runtimeHome := getRuntimeHome()
	platform, arch := platformArch()
	platformArch := platform + "_" + arch

	includeDir := filepath.Join(runtimeHome, "include")
	libDir := filepath.Join(runtimeHome, "lib", platformArch)
	headerTarget := filepath.Join(includeDir, headerName)

	if err := os.MkdirAll(includeDir, 0o755); err != nil {
		fatal("mkdir include: %v", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		fatal("mkdir lib: %v", err)
	}

	// Determine strategy: Windows or --static always use archive
	useArchive := platform == "windows" || *static
	var libTarget string

	if useArchive {
		archivePath := filepath.Join(libDir, archiveName)
		if err := downloadIfNeeded(baseURL+"/"+archiveName, archivePath, *force); err != nil {
			fatal("archive: %v", err)
		}
		if err := extractFromArchive(archivePath, "include/"+headerName, headerTarget, *force); err != nil {
			fatal("extract header: %v", err)
		}
		libTarget = filepath.Join(libDir, "liblancedb_go.a")
		if err := extractFromArchive(archivePath, "lib/"+platformArch+"/liblancedb_go.a", libTarget, *force); err != nil {
			fatal("extract lib: %v", err)
		}
		if platform == "windows" {
			fmt.Println("==> note: windows uses static library from release archive")
		}
	} else {
		// Direct download for darwin/linux dynamic libs
		headerURL := baseURL + "/" + headerName
		if err := downloadIfNeeded(headerURL, headerTarget, *force); err != nil {
			fatal("header: %v", err)
		}
		var assetName string
		switch platform {
		case "darwin":
			assetName = "liblancedb_go.dylib"
		case "linux":
			assetName = "liblancedb_go.so"
		default:
			fatal("unsupported platform: %s", platform)
		}
		libTarget = filepath.Join(libDir, assetName)
		if err := downloadIfNeeded(baseURL+"/"+assetName, libTarget, *force); err != nil {
			fatal("library: %v", err)
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

func downloadIfNeeded(url, target string, force bool) error {
	if !force {
		if _, err := os.Stat(target); err == nil {
			fmt.Println("==> reuse", filepath.Base(target))
			return nil
		}
	}
	fmt.Println("==> downloading", filepath.Base(target))
	return download(url, target)
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
