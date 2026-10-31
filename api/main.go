package main

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/storage"

	"github.com/ras251093/hgn-bowls/api/admins"
	"github.com/ras251093/hgn-bowls/api/auth"
	"github.com/ras251093/hgn-bowls/api/backup"
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
	switch arg(1) {
	case "migrate":
		err = migrate(ctx, logger)
	case "backup":
		err = backupPosts(ctx, logger)
	case "restore":
		err = restorePosts(ctx, logger, arg(2))
	default:
		err = run(ctx, logger)
	}
	if err != nil {
		logger.Error("stopped", "err", err)
		os.Exit(1)
	}
}

func arg(i int) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return ""
}

// openDatabase connects with TURSO_URL and TURSO_TOKEN, for the commands
// that only need the database.
func openDatabase() (*sql.DB, error) {
	url := os.Getenv("TURSO_URL")
	if url == "" {
		return nil, errors.New("missing required env vars: TURSO_URL")
	}
	return db.Open(url, os.Getenv("TURSO_TOKEN"))
}

// backupPosts exports every post to the private backups bucket. A Cloud
// Run job runs it weekly as "api backup"; the bucket deletes exports after
// 90 days.
func backupPosts(ctx context.Context, logger *slog.Logger) error {
	bucket := os.Getenv("BACKUP_BUCKET")
	if bucket == "" {
		return errors.New("missing required env vars: BACKUP_BUCKET")
	}
	conn, err := openDatabase()
	if err != nil {
		return err
	}
	defer conn.Close()
	gcs, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage client: %w", err)
	}
	defer gcs.Close()

	now := time.Now()
	name := backup.ObjectName(now)
	w := gcs.Bucket(bucket).Object(name).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	w.ContentType = "application/json"
	if err := backup.Export(ctx, db.New(conn), w, now); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("upload backup: %w", err)
	}
	logger.Info("backup written", "object", "gs://"+bucket+"/"+name)
	return nil
}

// restorePosts loads a backup file into an empty database, as
// "api restore FILE". Point TURSO_URL at file:… to try one locally; a file
// database is migrated first.
func restorePosts(ctx context.Context, logger *slog.Logger, path string) error {
	if path == "" {
		return errors.New("usage: api restore BACKUP.json")
	}
	f, err := os.Open(path) //nolint:gosec // G304: the operator names the file
	if err != nil {
		return err
	}
	defer f.Close()
	conn, err := openDatabase()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	n, err := backup.Restore(ctx, conn, f)
	if err != nil {
		return err
	}
	logger.Info("backup restored", "posts", n)
	return nil
}

// migrate brings the database schema up to date. The deploy job runs it as
// "api migrate" before a new revision takes traffic, so it only needs the
// database settings.
func migrate(ctx context.Context, logger *slog.Logger) error {
	conn, err := openDatabase()
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

	verifier, err := auth.NewVerifier(ctx, "https://"+cfg.Auth0Domain+"/", cfg.Auth0Audience,
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

	// Left nil until Auth0 management is configured; the admins endpoints
	// then answer 503.
	var directory admins.Directory
	if missing := cfg.AdminManagementMissing(); len(missing) > 0 {
		logger.Warn("inviting admins is off", "missing", missing)
	} else {
		directory = admins.NewAuth0(&http.Client{Timeout: 10 * time.Second}, admins.Settings{
			Domain:       cfg.Auth0Domain,
			ClientID:     cfg.Auth0MgmtClientID,
			ClientSecret: cfg.Auth0MgmtClientSecret,
			RoleID:       cfg.Auth0AdminRoleID,
			AppClientID:  cfg.Auth0AdminClientID,
		})
	}

	handler := newHandler(routes{
		events:       events.NewHandler(cache, logger),
		posts:        postsHandler,
		uploads:      bucket.UploadHandler(logger),
		latestDeploy: pages.LatestHandler(logger),
		rebuild:      pages.RebuildHandler(logger),
		admins:       admins.NewHandler(directory, auth.CallerID, logger),
		admin:        verifier.RequirePermission("manage:posts"),
		manageAdmins: verifier.RequirePermission("manage:admins"),
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
