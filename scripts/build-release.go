package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type buildTarget struct {
	GOOS   string
	GOARCH string
}

var releaseTargets = []buildTarget{
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "windows", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
}

func main() {
	version := flag.String("version", "v0.0.0", "version to embed")
	commit := flag.String("commit", "unknown", "commit to embed")
	buildDate := flag.String("date", "unknown", "build date to embed")
	target := flag.String("target", "", "target in GOOS/GOARCH form")
	local := flag.Bool("local", false, "build only the current host target")
	clean := flag.Bool("clean", false, "remove dist and coverage output")
	flag.Parse()

	if *clean {
		must(os.RemoveAll("dist"))
		_ = os.Remove("coverage.out")
		return
	}

	targets, err := selectTargets(*target, *local)
	must(err)
	must(os.MkdirAll("dist", 0o755))

	var checksumFiles []string
	for _, t := range targets {
		binaryPath, err := buildTargetBinary(t, *version, *commit, *buildDate)
		must(err)
		checksumFiles = append(checksumFiles, binaryPath)
	}
	must(writeChecksums(checksumFiles))
}

func selectTargets(target string, local bool) ([]buildTarget, error) {
	if target != "" && local {
		return nil, fmt.Errorf("-target and -local cannot be used together")
	}
	if local {
		return []buildTarget{{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}}, nil
	}
	if target == "" {
		return releaseTargets, nil
	}
	parts := strings.Split(target, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("target must use GOOS/GOARCH form")
	}
	return []buildTarget{{GOOS: parts[0], GOARCH: parts[1]}}, nil
}

func buildTargetBinary(target buildTarget, version string, commit string, buildDate string) (string, error) {
	fmt.Printf("Building %s/%s...\n", target.GOOS, target.GOARCH)
	name := binaryName(target)
	binaryPath := filepath.Join("dist", name)
	_ = os.Remove(binaryPath)

	ldflags := strings.Join([]string{
		"-s",
		"-w",
		"-buildid=",
		"-X", "github.com/yuanboshe/llm-relay/internal/cmd.Version=" + version,
		"-X", "github.com/yuanboshe/llm-relay/internal/cmd.Commit=" + commit,
		"-X", "github.com/yuanboshe/llm-relay/internal/cmd.BuildDate=" + buildDate,
	}, " ")
	args := []string{
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", ldflags,
		"-o", binaryPath,
		"./cmd/llmrelay",
	}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func binaryName(target buildTarget) string {
	name := fmt.Sprintf("llmrelay-%s-%s", target.GOOS, target.GOARCH)
	if target.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func writeChecksums(paths []string) error {
	out, err := os.Create(filepath.Join("dist", "SHA256SUMS"))
	if err != nil {
		return err
	}
	defer out.Close()

	for _, path := range paths {
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s  %s\n", sum, filepath.Base(path)); err != nil {
			return err
		}
	}
	return nil
}

func sha256File(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, in); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
