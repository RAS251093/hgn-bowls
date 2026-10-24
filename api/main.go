package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/storage"

	"github.com/ras251093/hgn-bowls/api/auth"
	"github.com/ras251093/hgn-bowls/api/config"
	"github.com/ras251093/hgn-bowls/api/db"
	"github.com/ras251093/hgn-bowls/api/events"
	"github.com/ras251093/hgn-bowls/api/media"
	"github.com/ras251093/hgn-bowls/api/posts"
	"github.com/ras251093/hgn-bowls/api/site"
)

const eventsTTL = 60 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With("revision", cmp.Or(os.Getenv("K_REVISION"), "local"))

	ctx := context.Background()
	var err error
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		err = migrate(ctx, logger)
	} else {
		err = run(ctx, logger)
	}
	if err != nil {
		logger.Error("stopped", "err", err)
		os.Exit(1)
	}
}

// migrate brings the database schema up to date. The deploy job runs it as
// "api migrate" before a new revision takes traffic, so it only needs the
// database settings.
func migrate(ctx context.Context, logger *slog.Logger) error {
	url := os.Getenv("TURSO_URL")
	if url == "" {
		return errors.New("missing required env vars: TURSO_URL")
	}
	conn, err := db.Open(url, os.Getenv("TURSO_TOKEN"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	logger.Info("migrations applied")
	return nil
}

func run(ctx context.Context, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	conn, err := db.Open(cfg.TursoURL, cfg.TursoToken)
	if err != nil {
		return err
	}
	defer conn.Close()
	queries := db.New(conn)

	// Signs as the Cloud Run service account; no key file.
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage client: %w", err)
	}
	defer gcs.Close()

	verifier, err := auth.NewVerifier("https://"+cfg.Auth0Domain+"/", cfg.Auth0Audience,
		&http.Client{Timeout: 5 * time.Second}, logger)
	if err != nil {
		return err
	}
	buildToken, err := auth.BuildToken(cfg.BuildToken)
	if err != nil {
		return err
	}

	calendar := events.NewCalendar(&http.Client{Timeout: 5 * time.Second}, cfg.CalendarID, cfg.CalendarAPIKey)
	cache := events.NewCache(calendar, eventsTTL,
		events.WithSnapshots(db.NewSnapshots(queries)),
		events.WithLogger(logger),
	)
	pages := site.NewPages(&http.Client{Timeout: 10 * time.Second},
		cfg.PagesDeployHook, cfg.CloudflareAPIToken, cfg.CloudflareAccountID, cfg.PagesProject)
	bucket := media.NewBucket(gcs.Bucket(cfg.Bucket))
	postsHandler := posts.NewHandler(posts.NewService(queries), pages, bucket, auth.CallerID, logger)

	handler := newHandler(routes{
		events:       events.NewHandler(cache, logger),
		posts:        postsHandler,
		uploads:      bucket.UploadHandler(logger),
		latestDeploy: pages.LatestHandler(logger),
		rebuild:      pages.RebuildHandler(logger),
		admin:        verifier.RequirePermission("manage:posts"),
		build:        buildToken,
	})
	handler = withCORS(cfg.AllowedOrigins, handler)
	handler = withRequestLog(logger, cfg.GoogleCloudProject, handler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,   // slow-loris protection
		ReadTimeout:       10 * time.Second,  // full request read
		WriteTimeout:      30 * time.Second,  // response write (covers handler time)
		IdleTimeout:       120 * time.Second, // keep-alive connections
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	logger.Info("listening", "addr", srv.Addr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
