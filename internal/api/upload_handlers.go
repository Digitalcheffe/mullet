package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"image"
	_ "image/gif"  // registers the "gif" format with image.Decode
	_ "image/jpeg" // registers "jpeg"
	_ "image/png"  // registers "png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp" // registers "webp" -- not in the standard library
)

// maxUploadSize bounds a single upload -- generous enough for a real
// photo (a background image, not a thumbnail), small enough that a
// handful of admins uploading images can't quietly fill the disk.
const maxUploadSize = 8 << 20 // 8 MiB

// allowedImageFormats maps image.Decode's own format name (what it
// determined the file actually *is*, not what its extension or
// Content-Type header claimed) to the file extension saved under --
// deliberately not derived from client input, so a mislabeled or
// malicious upload can't choose its own saved extension.
var allowedImageFormats = map[string]string{
	"png":  ".png",
	"jpeg": ".jpg",
	"gif":  ".gif",
	"webp": ".webp",
}

type uploadResponse struct {
	URL string `json:"url"`
}

// handleUploadImage serves POST /api/admin/uploads: accepts one
// multipart "file" field, verifies it actually decodes as one of the
// supported image formats (rejecting a file that merely *claims* to be
// an image via its extension or Content-Type -- both are attacker-
// controlled and neither is trusted here), and saves it under a
// randomly generated filename in uploadsDir. The returned URL
// (`/uploads/{filename}`) is meant to be used as a theme's
// `background.value` directly.
func handleUploadImage(uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "file missing or too large (max 8 MiB)", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `missing "file" field`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "reading upload", http.StatusInternalServerError)
			return
		}

		_, format, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			http.Error(w, "not a valid image", http.StatusBadRequest)
			return
		}
		ext, ok := allowedImageFormats[format]
		if !ok {
			http.Error(w, "unsupported image format (use PNG, JPEG, GIF, or WebP)", http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var nameBytes [16]byte
		if _, err := rand.Read(nameBytes[:]); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		filename := hex.EncodeToString(nameBytes[:]) + ext
		if err := os.WriteFile(filepath.Join(uploadsDir, filename), data, 0o644); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(uploadResponse{URL: "/uploads/" + filename})
	}
}

// handleServeUpload serves GET /uploads/{name}: no auth, same as
// /api/data and /display/{slug} -- a display rendering an uploaded
// background image has no way to attach a Bearer token, so this has to
// be publicly reachable the same way the rest of the LAN-facing display
// surface is. `name` is only ever a flat, randomly generated filename
// this same package created (see handleUploadImage), so any path
// separator or traversal attempt is rejected outright rather than
// resolved -- there's never a legitimate reason for one.
func handleServeUpload(uploadsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(uploadsDir, name))
	}
}
