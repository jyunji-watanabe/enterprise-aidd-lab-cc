// Command expense is the single entry point of the expense system backend.
//
//	expense serve   [-addr :8080] [-db data/expense.db] [-static ../frontend/dist] [-seed]
//	expense seed    [-db data/expense.db]          seed an empty database
//	expense remind  [-db data/expense.db]          run the reminder batch once
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/httpapi"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/seed"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func usage() error {
	return errors.New("usage: expense <serve|seed|remind> [flags]")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func openService(ctx context.Context, dbPath string, logger *slog.Logger) (*service.Service, func(), error) {
	if dbPath != ":memory:" {
		// dbPath comes from the operator's own -db flag / EXPENSE_DB env, not from remote input.
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil { //nolint:gosec // G703: trusted operator-supplied path
			return nil, nil, err
		}
	}
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return nil, nil, err
	}
	svc := service.New(st, logger, os.Stdout)
	return svc, func() { _ = st.Close() }, nil
}

func run(args []string, logger *slog.Logger) error {
	if len(args) == 0 {
		return usage()
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	dbPath := fs.String("db", envOr("EXPENSE_DB", "data/expense.db"), "SQLite database file (or :memory:)")
	addr := fs.String("addr", envOr("EXPENSE_ADDR", ":8080"), "listen address (serve)")
	static := fs.String("static", envOr("EXPENSE_STATIC", ""), "directory of the built frontend to serve (serve)")
	doSeed := fs.Bool("seed", envOr("EXPENSE_SEED", "true") == "true", "seed an empty database on startup (serve)")
	secure := fs.Bool("secure-cookie", envOr("EXPENSE_SECURE_COOKIE", "false") == "true", "set Secure on the session cookie (serve)")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc, closeFn, err := openService(ctx, *dbPath, logger)
	if err != nil {
		return err
	}
	defer closeFn()

	switch cmd {
	case "seed":
		seeded, err := seed.IfEmpty(ctx, svc)
		if err != nil {
			return err
		}
		fmt.Printf("seeded=%v db=%s\n", seeded, *dbPath)
		return nil
	case "remind":
		res, err := svc.RunReminders(ctx, service.SystemActor, service.Meta{IP: "local", UserAgent: "cli"})
		if err != nil {
			return err
		}
		fmt.Printf("run=%s threshold=%s targets=%d reminders=%d\n", res.RunID, res.Threshold, res.Targets, len(res.Reminders))
		return nil
	case "serve":
		if *doSeed {
			seeded, err := seed.IfEmpty(ctx, svc)
			if err != nil {
				return err
			}
			if seeded {
				logger.Info("seeded initial data", "db", *dbPath)
			}
		}
		return serve(ctx, svc, logger, *addr, httpapi.Options{StaticDir: *static, SecureCookie: *secure})
	default:
		return usage()
	}
}

func serve(ctx context.Context, svc *service.Service, logger *slog.Logger, addr string, opts httpapi.Options) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(svc, logger, opts).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr, "static", opts.StaticDir)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
