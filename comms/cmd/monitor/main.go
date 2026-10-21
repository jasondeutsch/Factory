package main

import (
	"context"
	"factory/comms/internal/monitor"
	"flag"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
)

func main() {
	endpoint := os.Getenv("FACTORY_COMMS_URL")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8090/mcp"
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	project := flag.String("project", cwd, "Exact Comms project identifier (defaults to working directory)")
	address := flag.String("url", endpoint, "Comms URL; also accepts FACTORY_COMMS_URL")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Unexpected positional arguments; use -project or -url")
		os.Exit(1)
	}
	if canonical, err := filepath.EvalSymlinks(*project); err == nil {
		absolute, absErr := filepath.Abs(canonical)
		if absErr == nil {
			*project = absolute
		}
	}
	client, err := monitor.NewClient(*address, os.Getenv("COMMS_AUTH_TOKEN"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = tea.NewProgram(monitor.New(ctx, client, *project), tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Monitor:", err)
		os.Exit(1)
	}
}
