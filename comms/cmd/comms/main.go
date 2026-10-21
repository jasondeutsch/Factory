package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"factory/comms/internal/comms"
)

func setting(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("comms stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	address := flag.String("addr", setting("COMMS_ADDR", "127.0.0.1:8080"), "HTTP listening address")
	database := flag.String("db", setting("COMMS_DB_PATH", "/data/state.db"), "SQLite database path inside the container")
	flag.Parse()
	if flag.NArg() > 1 || (flag.NArg() == 1 && flag.Arg(0) != "health") {
		return errors.New("usage: comms [-addr address] [-db path] [health]")
	}
	if flag.NArg() == 1 {
		host, port, err := net.SplitHostPort(*address)
		if err != nil {
			return err
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		client := http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("health check: HTTP %d", response.StatusCode)
		}
		return nil
	}
	store, err := comms.Open(*database)
	if err != nil {
		return fmt.Errorf("open SQLite: %w", err)
	}
	defer store.Close()
	server := &http.Server{Addr: *address, Handler: comms.HTTPHandler(store, os.Getenv("COMMS_AUTH_TOKEN"), logger), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failure := make(chan error, 1)
	go func() {
		logger.Info("comms listening", "address", *address, "database", *database)
		failure <- server.ListenAndServe()
	}()
	select {
	case err := <-failure:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
