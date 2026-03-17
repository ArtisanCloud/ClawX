package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"clawx/internal/infrastructure/config"
)

type installServiceOptions struct {
	Name   string
	Start  bool
	Binary string
	Force  bool
}

func runInstallService(args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("install-service is only supported on Linux")
	}

	opts, err := parseInstallServiceArgs(args)
	if err != nil {
		return err
	}
	return installService(opts)
}

func installService(opts installServiceOptions) error {
	configPath, exists, err := config.Exists()
	if err != nil {
		return fmt.Errorf("check config: %w", err)
	}
	if !exists {
		return fmt.Errorf("config file %q not found; run `clawx config` first", configPath)
	}

	binaryPath, err := resolveServiceBinaryPath(opts.Binary)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	unitName := normalizeServiceName(opts.Name)
	unitPath := filepath.Join(home, ".config", "systemd", "user", unitName+".service")
	if !opts.Force {
		if _, err := os.Stat(unitPath); err == nil {
			return fmt.Errorf("service unit already exists: %s (use --force to overwrite)", unitPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat service unit: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("create service unit dir: %w", err)
	}

	content := buildSystemdUserUnit(unitName, binaryPath, configPath, home)
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}

	if err := runSystemctlUser("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctlUser("enable", unitName+".service"); err != nil {
		return err
	}
	if opts.Start {
		if err := runSystemctlUser("restart", unitName+".service"); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stdout, "installed user service: %s\n", unitPath)
	fmt.Fprintf(os.Stdout, "enabled: systemctl --user enable %s.service\n", unitName)
	if opts.Start {
		fmt.Fprintf(os.Stdout, "started: systemctl --user restart %s.service\n", unitName)
	} else {
		fmt.Fprintf(os.Stdout, "to start now: systemctl --user start %s.service\n", unitName)
	}
	fmt.Fprintf(os.Stdout, "to check status: systemctl --user status %s.service\n", unitName)
	return nil
}

func parseInstallServiceArgs(args []string) (installServiceOptions, error) {
	fs := flag.NewFlagSet("install-service", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	opts := installServiceOptions{}
	fs.StringVar(&opts.Name, "name", "clawx", "user systemd service name")
	fs.BoolVar(&opts.Start, "start", false, "start service immediately after install")
	fs.StringVar(&opts.Binary, "binary", "", "absolute path to clawx binary (required when using go run)")
	fs.BoolVar(&opts.Force, "force", false, "overwrite existing unit file")
	if err := fs.Parse(args); err != nil {
		return installServiceOptions{}, err
	}
	if extra := fs.Args(); len(extra) > 0 {
		return installServiceOptions{}, fmt.Errorf("unexpected args: %s", strings.Join(extra, " "))
	}
	return opts, nil
}

func resolveServiceBinaryPath(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		path := filepath.Clean(strings.TrimSpace(override))
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("--binary must be an absolute path")
		}
		if err := ensureExecutableFile(path); err != nil {
			return "", fmt.Errorf("invalid --binary path: %w", err)
		}
		return path, nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	cleaned := filepath.Clean(execPath)
	if strings.Contains(cleaned, string(filepath.Separator)+"go-build"+string(filepath.Separator)) {
		return "", fmt.Errorf("current executable is a temporary go-run binary; provide --binary /abs/path/to/clawx")
	}
	if err := ensureExecutableFile(cleaned); err != nil {
		return "", fmt.Errorf("invalid executable path %q: %w", cleaned, err)
	}
	return cleaned, nil
}

func ensureExecutableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("path is not executable")
	}
	return nil
}

func normalizeServiceName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return "clawx"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-_")
	if result == "" {
		return "clawx"
	}
	return result
}

func buildSystemdUserUnit(serviceName, binaryPath, configPath, home string) string {
	escapedBinary := systemdEscape(binaryPath)
	escapedConfig := systemdEscape(configPath)
	escapedHome := systemdEscape(home)
	escapedWorkDir := systemdEscape(home)

	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=ClawX Service (")
	b.WriteString(serviceName)
	b.WriteString(")\n")
	b.WriteString("After=network.target\n\n")

	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("WorkingDirectory=")
	b.WriteString(escapedWorkDir)
	b.WriteString("\n")
	b.WriteString("Environment=HOME=")
	b.WriteString(escapedHome)
	b.WriteString("\n")
	b.WriteString("Environment=CLAWX_CONFIG=")
	b.WriteString(escapedConfig)
	b.WriteString("\n")
	b.WriteString("ExecStart=")
	b.WriteString(escapedBinary)
	b.WriteString(" run\n")
	b.WriteString("Restart=always\n")
	b.WriteString("RestartSec=3\n\n")

	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=default.target\n")
	return b.String()
}

func systemdEscape(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.ContainsAny(value, " \t") {
		return strconvQuote(value)
	}
	return value
}

func strconvQuote(value string) string {
	// Avoid pulling in strconv in callers repeatedly.
	return fmt.Sprintf("%q", value)
}

func runSystemctlUser(args ...string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required but not found in PATH")
	}
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("systemctl --user %s failed: %s", strings.Join(args, " "), message)
	}
	return nil
}
