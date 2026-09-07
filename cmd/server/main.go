// Command server is the Mullet dashboard server entry point.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Digitalcheffe/mullet/internal/api"
	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/config"
	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/scheduler"
)

func main() {
	startedAt := time.Now()
	cfg := config.Load()

	sqldb, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	if err := db.Migrate(sqldb); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := plugindata.NewRegistry()
	sched := scheduler.New(sqldb, registry)
	if err := sched.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer sched.Stop()

	jwtSecret, err := auth.LoadOrCreateJWTSecret(sqldb)
	if err != nil {
		log.Fatal(err)
	}

	router := api.NewRouter(sqldb, jwtSecret, cfg.CORSOrigins, api.ServerInfo{
		Port:      cfg.Port,
		DBPath:    cfg.DBPath,
		StartedAt: startedAt,
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("mullet server listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
