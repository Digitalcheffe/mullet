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

	// Embeds the IANA time zone database in the binary, so TZID lookups
	// (e.g. the ICS feed plugin resolving VTIMEZONE) work even on a
	// minimal runtime image with no /usr/share/zoneinfo (our Alpine
	// Docker image doesn't install the tzdata package).
	_ "time/tzdata"

	"github.com/Digitalcheffe/mullet/internal/api"
	"github.com/Digitalcheffe/mullet/internal/auth"
	"github.com/Digitalcheffe/mullet/internal/config"
	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"

	// Compiled-in data plugins register themselves via init(). Adding a
	// new plugin means implementing DataPlugin and blank-importing its
	// package here.
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/clock"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/homeassistant"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/icsfeed"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/msgraphcalendar"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/msgraphtodo"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/openmeteo"
	_ "github.com/Digitalcheffe/mullet/internal/plugins/data/openweathermap"

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

	sched := scheduler.New(sqldb, plugindata.Default)
	if err := sched.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer sched.Stop()

	jwtSecret, err := auth.LoadOrCreateJWTSecret(sqldb)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.AuthDisabled {
		log.Println("WARNING: AUTH_DISABLED=true -- the admin API and UI require no login. Local dev only; never set this in a real deployment.")
	}

	router := api.NewRouter(sqldb, jwtSecret, cfg.CORSOrigins, api.ServerInfo{
		Port:      cfg.Port,
		DBPath:    cfg.DBPath,
		StartedAt: startedAt,
	}, cfg.StaticDir, cfg.AuthDisabled, plugindata.Default, sched, cfg.UploadsDir)
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
