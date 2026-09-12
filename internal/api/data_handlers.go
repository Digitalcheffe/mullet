package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

type dataResponse struct {
	Data        []map[string]any `json:"data"`
	LastUpdated string           `json:"last_updated"`
	Source      string           `json:"source"`
}

// handleGetShapeData serves GET /api/data/{shape}, reading directly from
// the shape's typed table -- it never calls a plugin. Optional query
// params: ?plugin={instance_id}, repeatable, scopes to one or more
// plugin instances -- issue #97's multi-source calendar cards pass
// several so a card can merge events from more than one calendar
// source into one sorted response; ?from=&to= (RFC3339) range-filter
// shapes that support it (currently just events).
func handleGetShapeData(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shape := r.PathValue("shape")
		if !db.IsValidShape(shape) {
			http.Error(w, fmt.Sprintf("unknown shape %q", shape), http.StatusNotFound)
			return
		}

		var pluginInstanceIDs []int
		for _, raw := range r.URL.Query()["plugin"] {
			id, err := strconv.Atoi(raw)
			if err != nil {
				http.Error(w, "plugin must be a numeric instance id", http.StatusBadRequest)
				return
			}
			pluginInstanceIDs = append(pluginInstanceIDs, id)
		}

		from, to, err := parseTimeRange(r, shape)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		rows, sources, lastUpdated, err := db.ReadShape(sqldb, shape, pluginInstanceIDs, from, to)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := dataResponse{
			Data:   rows,
			Source: strings.Join(sources, ","),
		}
		if lastUpdated != nil {
			resp.LastUpdated = *lastUpdated
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func parseTimeRange(r *http.Request, shape string) (from, to *time.Time, err error) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	if fromRaw == "" && toRaw == "" {
		return nil, nil, nil
	}
	if !db.SupportsTimeRange(shape) {
		return nil, nil, fmt.Errorf("shape %q does not support from/to filtering", shape)
	}

	if fromRaw != "" {
		t, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("from must be an RFC3339 timestamp")
		}
		from = &t
	}
	if toRaw != "" {
		t, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("to must be an RFC3339 timestamp")
		}
		to = &t
	}
	return from, to, nil
}
