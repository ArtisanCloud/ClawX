package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"clawx/internal/infrastructure/config"
	stdlogging "clawx/internal/infrastructure/logging"
)

func runTraceCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("缺少 trace 子命令。示例：`clawx trace cache-report`")
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "cache-report":
		return runTraceCacheReport(args[1:], os.Stdout)
	case "token-report":
		return runTraceTokenReport(args[1:], os.Stdout)
	case "help", "-h", "--help":
		printTraceUsage()
		return nil
	default:
		return fmt.Errorf("未知 trace 子命令 %q", strings.TrimSpace(args[0]))
	}
}

func runTraceCacheReport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trace cache-report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("file", "", "trace jsonl file path")
	jsonOutput := fs.Bool("json", false, "print json output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	traceFile := strings.TrimSpace(*file)
	if traceFile == "" {
		traceFile = filepath.Join(config.StateDir(), "logs", "trace.jsonl")
	}
	f, err := os.Open(traceFile)
	if err != nil {
		return fmt.Errorf("open trace file: %w", err)
	}
	defer f.Close()

	metrics, err := stdlogging.AggregatePromptCacheMetrics(f)
	if err != nil {
		return fmt.Errorf("aggregate prompt cache metrics: %w", err)
	}
	if *jsonOutput {
		body, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal metrics json: %w", err)
		}
		if _, err := stdout.Write(append(body, '\n')); err != nil {
			return err
		}
		return nil
	}
	_, err = fmt.Fprintln(stdout, stdlogging.RenderPromptCacheMetricsReport(metrics))
	return err
}

func printTraceUsage() {
	fmt.Println("Trace 命令：")
	fmt.Println("- clawx trace cache-report [--file <trace.jsonl>] [--json]")
	fmt.Println("- clawx trace token-report [--file <token_usage.jsonl>] [--json]")
}

func runTraceTokenReport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trace token-report", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	file := fs.String("file", "", "token usage jsonl file path")
	jsonOutput := fs.Bool("json", false, "print json output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tokenFile := strings.TrimSpace(*file)
	if tokenFile == "" {
		tokenFile = filepath.Join(config.StateDir(), "logs", "token_usage.jsonl")
	}
	summary, err := summarizeTokenUsageFile(tokenFile)
	if err != nil {
		return fmt.Errorf("summarize token usage: %w", err)
	}
	if *jsonOutput {
		body, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal token summary json: %w", err)
		}
		if _, err := stdout.Write(append(body, '\n')); err != nil {
			return err
		}
		return nil
	}
	_, err = fmt.Fprintln(stdout, renderTokenUsageSummary(summary))
	return err
}
