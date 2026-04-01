package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"clawx/internal/infrastructure/config"
)

func runStartupExecutionSelfCheck(cfg config.Snapshot) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CLAWX_STARTUP_SELF_CHECK")), "false") ||
		strings.TrimSpace(os.Getenv("CLAWX_STARTUP_SELF_CHECK")) == "0" {
		log.Printf("startup self-check skipped: CLAWX_STARTUP_SELF_CHECK=%q", strings.TrimSpace(os.Getenv("CLAWX_STARTUP_SELF_CHECK")))
		return
	}

	cwd, _ := os.Getwd()
	log.Printf("startup self-check begin: executor=clawx-process pid=%d cwd=%s default_cwd=%s allowed_roots=%d",
		os.Getpid(),
		strings.TrimSpace(cwd),
		strings.TrimSpace(cfg.DefaultCWD),
		len(cfg.AllowedRoots),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	runCheck("dns.pypi.org", func() (string, error) {
		addrs, err := net.DefaultResolver.LookupHost(ctx, "pypi.org")
		if err != nil {
			return "", err
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("no addresses resolved")
		}
		sort.Strings(addrs)
		limit := len(addrs)
		if limit > 3 {
			limit = 3
		}
		return strings.Join(addrs[:limit], ", "), nil
	})

	runCheck("https.pypi.simple", func() (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://pypi.org/simple/", nil)
		if err != nil {
			return "", err
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		return fmt.Sprintf("status=%d", resp.StatusCode), nil
	})

	runCheck("python3.version", func() (string, error) {
		out, err := exec.CommandContext(ctx, "python3", "--version").CombinedOutput()
		return compactOutput(out), err
	})

	runCheck("pip.version", func() (string, error) {
		out, err := exec.CommandContext(ctx, "python3", "-m", "pip", "--version").CombinedOutput()
		return compactOutput(out), err
	})

	runCheck("pip.index", func() (string, error) {
		cmdCtx, cmdCancel := context.WithTimeout(ctx, 10*time.Second)
		defer cmdCancel()
		out, err := exec.CommandContext(cmdCtx, "bash", "-lc", "python3 -m pip index versions pip | head -n 3").CombinedOutput()
		return compactOutput(out), err
	})

	log.Printf("startup self-check done")
}

func runCheck(name string, fn func() (string, error)) {
	detail, err := fn()
	if err != nil {
		log.Printf("startup self-check fail: %s err=%v detail=%s", name, err, strings.TrimSpace(detail))
		return
	}
	log.Printf("startup self-check ok: %s %s", name, strings.TrimSpace(detail))
}

func compactOutput(out []byte) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(string(out))), " ")
	if text == "" {
		return "(no output)"
	}
	if len(text) <= 180 {
		return text
	}
	return text[:180] + "..."
}
