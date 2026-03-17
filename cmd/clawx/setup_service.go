package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"clawx/internal/infrastructure/config"
)

type setupServiceOptions struct {
	Target    string
	Binary    string
	Name      string
	Start     bool
	Force     bool
	SkipBuild bool
}

func runSetupService(args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("setup-service is only supported on Linux")
	}
	opts, err := parseSetupServiceArgs(args)
	if err != nil {
		return err
	}

	if _, _, err := config.EnsureDefaultFile(); err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}
	if _, err := config.SetValueByDotKey("service.run", opts.Target); err != nil {
		return fmt.Errorf("set service.run: %w", err)
	}

	if !opts.SkipBuild {
		if err := buildClawxBinary(opts.Binary); err != nil {
			return err
		}
	}

	return installService(installServiceOptions{
		Name:   opts.Name,
		Start:  opts.Start,
		Binary: opts.Binary,
		Force:  opts.Force,
	})
}

func parseSetupServiceArgs(args []string) (setupServiceOptions, error) {
	fs := flag.NewFlagSet("setup-service", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	home, _ := os.UserHomeDir()
	defaultBinary := filepath.Join(strings.TrimSpace(home), ".local", "bin", "clawx")

	opts := setupServiceOptions{
		Target: "serve",
		Binary: defaultBinary,
		Name:   "clawx",
		Start:  true,
		Force:  true,
	}
	fs.StringVar(&opts.Target, "target", opts.Target, "run target: serve|telegram|discord|feishu|wecom")
	fs.StringVar(&opts.Binary, "binary", opts.Binary, "absolute output path for clawx binary")
	fs.StringVar(&opts.Name, "name", opts.Name, "user systemd service name")
	fs.BoolVar(&opts.Start, "start", opts.Start, "start service immediately after install")
	fs.BoolVar(&opts.Force, "force", opts.Force, "overwrite existing unit file")
	fs.BoolVar(&opts.SkipBuild, "skip-build", false, "skip go build and use existing --binary")
	if err := fs.Parse(args); err != nil {
		return setupServiceOptions{}, err
	}
	if extra := fs.Args(); len(extra) > 0 {
		return setupServiceOptions{}, fmt.Errorf("unexpected args: %s", strings.Join(extra, " "))
	}
	opts.Target = normalizeRunTarget(opts.Target)
	if opts.Target == "" {
		return setupServiceOptions{}, fmt.Errorf("invalid --target; expected serve|telegram|discord|feishu|wecom")
	}
	opts.Binary = strings.TrimSpace(opts.Binary)
	if !filepath.IsAbs(opts.Binary) {
		return setupServiceOptions{}, fmt.Errorf("--binary must be an absolute path")
	}
	return opts, nil
}

func buildClawxBinary(binaryPath string) error {
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return fmt.Errorf("create binary dir: %w", err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/clawx")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	return nil
}
