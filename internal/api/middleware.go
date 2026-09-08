package api

import (
	"context"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Digitalcheffe/mullet/internal/auth"
)

type contextKey string

const claimsContextKey contextKey = "claims"

// claimsFromContext returns the authenticated user's claims, set by
// requireAuth for requests that passed it.
func claimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*auth.Claims)
	return claims, ok
}

// withLogging logs method, path, status code, and duration for every
// request.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// withCORS sets CORS headers for requests whose Origin is in
// allowedOrigins (or for every origin if allowedOrigins contains "*"),
// and short-circuits preflight OPTIONS requests. With no allowed origins
// configured, it's a no-op passthrough.
func withCORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (slices.Contains(allowedOrigins, "*") || slices.Contains(allowedOrigins, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// devClaims stands in for a real session when authDisabled bypasses the
// token check entirely.
var devClaims = &auth.Claims{UserID: 0, Username: "dev"}

// requireAuth rejects requests without a valid "Authorization: Bearer
// <jwt>" header with 401, and otherwise attaches the token's claims to
// the request context for downstream handlers. With authDisabled, every
// request passes through unauthenticated -- local dev only, never set
// this in a real deployment.
func requireAuth(secret []byte, authDisabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled {
				ctx := context.WithValue(r.Context(), claimsContextKey, devClaims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			tokenString, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || tokenString == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := auth.ParseToken(secret, tokenString)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
