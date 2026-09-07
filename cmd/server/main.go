// Command server is the Mullet dashboard server entry point.
package main

import (
	"log"
	"net/http"

	"github.com/Digitalcheffe/mullet/internal/api"
	"github.com/Digitalcheffe/mullet/internal/config"
)

func main() {
	cfg := config.Load()

	router := api.NewRouter()

	addr := ":" + cfg.Port
	log.Printf("mullet server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
