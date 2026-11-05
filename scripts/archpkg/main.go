package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "archpkg:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: archpkg <linux-build-dir>")
	}
	stage := args[0]
	cfg, err := loadApp(stage)
	if err != nil {
		return err
	}
	name, err := pkgName(cfg, stage)
	if err != nil {
		return err
	}
	tarBytes, err := packTar(stage, cfg, time.Now())
	if err != nil {
		return err
	}
	out := filepath.Join(filepath.Dir(stage), name)
	cmd := exec.Command("zstd", "-f", "-q", "-19", "-o", out, "-")
	cmd.Stdin = bytes.NewReader(tarBytes)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zstd: %v\n%s", err, msg)
	}
	fmt.Println(out)
	return nil
}

// loadApp reads mygo.json from the project that contains stage.
func loadApp(stage string) (appInfo, error) {
	dir, err := filepath.Abs(stage)
	if err != nil {
		return appInfo{}, err
	}
	for {
		path := filepath.Join(dir, "mygo.json")
		b, err := os.ReadFile(path)
		if err == nil {
			var app appInfo
			if err := json.Unmarshal(b, &app); err != nil {
				return appInfo{}, fmt.Errorf("%s: %w", path, err)
			}
			if app.Slug == "" || app.Version == "" {
				return appInfo{}, fmt.Errorf("%s: needs slug and version", path)
			}
			return app, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return appInfo{}, fmt.Errorf("mygo.json not found above %s", stage)
		}
		dir = parent
	}
}
