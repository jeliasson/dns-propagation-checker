package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeliasson/dns-propagation-checker/internal/resolve"
	"github.com/jeliasson/dns-propagation-checker/internal/server"
)

func runServe(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	addr := fs.String("addr", envOr("DNSPC_ADDR", ":3000"), "listen address")
	rateBurst := fs.Int("rate-burst", 20, "requests allowed per rate window, per client")
	rateWindow := fs.Duration("rate-window", time.Minute, "rate limit window")
	timeout := fs.Duration("timeout", resolve.DefaultTimeout, "per-query DNS timeout")
	trustProxy := fs.Bool("trust-proxy", os.Getenv("DNSPC_TRUST_PROXY") != "",
		"read the client IP from X-Forwarded-For (only behind a proxy you control)")
	logLevel := fs.String("log-level", envOr("DNSPC_LOG_LEVEL", "info"), "debug, info, warn or error")

	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: dnspc serve [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	level, err := parseLevel(*logLevel)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	app := server.New(server.Config{
		RateBurst:    *rateBurst,
		RateWindow:   *rateWindow,
		QueryTimeout: *timeout,
		TrustProxy:   *trustProxy,
		Logger:       logger,
	})
	defer app.Close()

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		// Long enough for the slowest query plus its own timeout margin.
		WriteTimeout: *timeout + 30*time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", *addr, "version", Version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
		close(errs)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

func parseLevel(name string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return 0, fmt.Errorf("invalid log level %q", name)
	}
	return level, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
