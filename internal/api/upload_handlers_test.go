package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/auth"
)

// pngBytes returns a minimal valid 1x1 PNG, encoded fresh each call so
// the test doesn't depend on a fixture file living anywhere in the repo.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

func multipartUploadRequest(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing form file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}
	return &body, w.FormDataContentType()
}

func TestUploadImageAcceptsValidPNGAndServesItBack(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, contentType := multipartUploadRequest(t, "file", "background.png", pngBytes(t))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/uploads", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+mustIssueUploadTestToken(t))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding upload response: %v", err)
	}
	if resp.URL == "" {
		t.Fatal("upload response has no url")
	}

	// The returned URL must actually serve the uploaded image back,
	// unauthenticated -- a display rendering it as a theme background
	// has no way to attach a Bearer token.
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, resp.URL, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", resp.URL, getRec.Code)
	}
	if !bytes.Equal(getRec.Body.Bytes(), pngBytes(t)) {
		t.Error("served file content doesn't match what was uploaded")
	}
}

func TestUploadImageRejectsNonImageContent(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, contentType := multipartUploadRequest(t, "file", "not-an-image.png", []byte("this is just plain text, not a PNG"))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/uploads", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+mustIssueUploadTestToken(t))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (content claims .png but doesn't decode as one)", rec.Code)
	}
}

func TestUploadImageRequiresAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, contentType := multipartUploadRequest(t, "file", "background.png", pngBytes(t))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/uploads", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestServeUploadRejectsPathTraversal(t *testing.T) {
	handler := handleServeUpload(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/uploads/whatever", nil)
	req.SetPathValue("name", "../../go.mod")

	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (traversal attempt must not resolve)", rec.Code)
	}
}

// mustIssueUploadTestToken mints a JWT the same way authedRequest does
// -- duplicated (rather than reusing authedRequest itself) because a
// multipart upload needs its own request body construction, unlike
// authedRequest's always-JSON body.
func mustIssueUploadTestToken(t *testing.T) string {
	t.Helper()
	token, err := auth.IssueToken([]byte(testJWTSecret), 1, "admin")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	return token
}
