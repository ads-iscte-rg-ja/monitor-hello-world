// monitor-hello-world exercises the thin SDK against a remote-prepared buffer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	monitor "github.com/rmg-unicage/monitor-go-sdk"
)

func main() {
	scenario := flag.String("scenario", "logs", "logs or panic (delivery belongs to monitor-remote)")
	path := flag.String("db", env("MONITOR_DB_PATH", "./monitor.db"), "SQLite file already initialized by monitor-remote")
	count := flag.Int("count", 1, "copies of each log level (1-1000)")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, *scenario, *path, *count); err != nil {
		slog.Error("scenario failed", "error", err)
		os.Exit(1)
	}
	slog.Info("records queued locally; monitor-remote owns delivery", "scenario", *scenario)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func run(ctx context.Context, scenario, path string, count int) error {
	if count < 1 || count > 1000 {
		return fmt.Errorf("count must be between 1 and 1000")
	}
	db, err := monitor.ConnectDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	switch scenario {
	case "logs":
		for i := 0; i < count; i++ {
			for _, level := range []string{"INFO", "WARN", "ERROR", "CRITICAL", "PANIC"} {
				details := monitor.ErrorDetails{Metadata: json.RawMessage(`{"simulated":true}`)}
				if level == "CRITICAL" || level == "PANIC" {
					stack := string(debug.Stack())
					details.StackTrace = &stack
				}
				if _, err := monitor.LogError(ctx, db, "hello-world", "testing", level, "Simulated "+strings.ToLower(level), details); err != nil {
					return err
				}
			}
		}
		return nil
	case "panic":
		return capturePanic(func(message, stack string) error {
			_, err := monitor.LogError(ctx, db, "hello-world", "testing", "PANIC", message, monitor.ErrorDetails{StackTrace: &stack})
			return err
		})
	default:
		return fmt.Errorf("unknown scenario %q; supported: logs, panic", scenario)
	}
}

func capturePanic(record func(message, stack string) error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = record(fmt.Sprint(value), string(debug.Stack()))
		}
	}()
	panic("intentional tester panic (recovered and buffered)")
}
