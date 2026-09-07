// Command server is the Mullet dashboard server entry point.
package main

import (
	"log"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/api"
	"github.com/Digitalcheffe/mullet/internal/config"
	"github.com/Digitalcheffe/mullet/internal/db"
)

func main() {
	cfg := config.Load()

	sqldb, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	if err := db.Migrate(sqldb); err != nil {
		log.Fatal(err)
	}

	router := api.NewRouter()

	addr := ":" + cfg.Port
	log.Printf("mullet server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
