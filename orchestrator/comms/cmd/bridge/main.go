// The bridge runs on the host, inside Herdr. Comms remains runner-neutral.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"factory/comms/internal/delivery"
)

func main() {
	if err := run(); err != nil {
		slog.Error("delivery bridge stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	project := flag.String("project", "", "Exact project identifier registered in comms (required for delivery)")
	task := flag.String("task", "", "Optional explicit task ID; otherwise watch all project tasks")
	url := flag.String("url", os.Getenv("FACTORY_COMMS_URL"), "Comms MCP URL; defaults to http://127.0.0.1:8090/mcp")
	bind := flag.String("bind", "", "Print session_ref JSON for an explicit live Herdr agent name, then exit")
	notify := flag.String("notify", "", "Notify an active task routing identity once, using --agent and --task")
	agent := flag.String("agent", "", "Actual live Herdr agent name for --notify")
	once := flag.Bool("once", false, "Run one polling cycle and exit")
	interval := flag.Duration("interval", 3*time.Second, "Polling interval (minimum one second)")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if os.Getenv("HERDR_ENV") != "1" {
		return errors.New("start the bridge inside a Herdr-managed pane; do not override HERDR_ENV outside Herdr")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	host := delivery.Herdr{}
	if *bind != "" {
		binding, err := host.Bind(ctx, *bind)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(binding)
	}
	if *project == "" {
		return errors.New("--project is required; use the exact comms project identifier")
	}
	if *interval < time.Second {
		return errors.New("--interval must be at least one second")
	}
	if *url == "" {
		*url = "http://127.0.0.1:8090/mcp"
	}
	if *notify != "" || *agent != "" {
		if *notify == "" || *agent == "" || *task == "" {
			return errors.New("--notify requires --agent, --task and --project")
		}
		binding, err := host.Bind(ctx, *agent)
		if err != nil {
			return err
		}
		client, err := delivery.Connect(ctx, *url, os.Getenv("COMMS_AUTH_TOKEN"))
		if err != nil {
			return err
		}
		defer client.Close()
		bridge := delivery.Bridge{Client: client, Host: host, Project: *project, TaskID: *task, Logger: slog.Default()}
		return bridge.Notify(ctx, *notify, binding)
	}
	delay := *interval
	for ctx.Err() == nil {
		client, err := delivery.Connect(ctx, *url, os.Getenv("COMMS_AUTH_TOKEN"))
		if err == nil {
			bridge := delivery.Bridge{Client: client, Host: host, Project: *project, TaskID: *task, Logger: slog.Default()}
			for ctx.Err() == nil {
				err = bridge.Cycle(ctx)
				if err != nil || *once {
					break
				}
				delay = *interval
				select {
				case <-ctx.Done():
				case <-time.After(*interval):
				}
			}
			client.Close()
		}
		if *once {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		slog.Error("comms connection/cycle failed; reconnecting", "error", err, "retry_in", delay.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
	return fmt.Errorf("bridge cancelled: %w", ctx.Err())
}
