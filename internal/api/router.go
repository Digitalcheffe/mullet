// Package api wires up the HTTP router serving the data API, admin API,
// client API, and display endpoints.
package api

import "net/http"

// NewRouter builds the top-level HTTP handler for the server.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
