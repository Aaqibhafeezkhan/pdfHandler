package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func init() { gin.SetMode(gin.TestMode) }

// testRouter mounts the handlers on the same paths main uses. The middleware is
// deliberately absent — it is covered by the tests in package main.
func testRouter() *gin.Engine {
	r := gin.New()
	g := r.Group("/api")
	g.POST("/merge", Merge)
	g.POST("/split", Split)
	g.POST("/compress", Compress)
	g.POST("/rotate", Rotate)
	g.POST("/pdf-to-image", PDFToImage)
	g.POST("/image-to-pdf", ImageToPDF)
	g.POST("/protect", Protect)
	g.POST("/unlock", Unlock)
	g.POST("/preview", Preview)
	return r
}

// upload is one file part of a multipart request.
type upload struct {
	field    string
	filename string
	data     []byte
}

// post builds a multipart request and runs it through the router.
func post(t *testing.T, r http.Handler, path string, uploads []upload, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	for _, u := range uploads {
		// CreateFormFile escapes the filename, which would defeat the traversal
		// tests — write the part header directly so the raw name reaches the
		// handler exactly as a hostile client would send it.
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`, u.field, u.filename),
		}
		h["Content-Type"] = []string{"application/octet-stream"}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("creating part %q: %v", u.filename, err)
		}
		if _, err := part.Write(u.data); err != nil {
			t.Fatalf("writing part %q: %v", u.filename, err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("writing field %q: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// pngBytes renders a small solid-colour PNG.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			img.Set(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 6), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding png: %v", err)
	}
	return buf.Bytes()
}

// pdfBytes builds a genuine n-page PDF via pdfcpu, so the fixtures exercise the
// real parser instead of a hand-written stub, and no external tool is needed.
func pdfBytes(t *testing.T, pages int) []byte {
	t.Helper()

	dir := t.TempDir()
	imgData := pngBytes(t)

	var imgs []string
	for i := range pages {
		p := filepath.Join(dir, fmt.Sprintf("src_%d.png", i))
		if err := os.WriteFile(p, imgData, 0o600); err != nil {
			t.Fatalf("writing fixture image: %v", err)
		}
		imgs = append(imgs, p)
	}

	out := filepath.Join(dir, "fixture.pdf")
	if err := api.ImportImagesFile(imgs, out, nil, nil); err != nil {
		t.Fatalf("building fixture pdf: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading fixture pdf: %v", err)
	}
	return data
}

// pageCount reports how many pages a PDF in memory has.
func pageCount(t *testing.T, data []byte) int {
	t.Helper()
	n, err := api.PageCount(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("counting pages: %v", err)
	}
	return n
}

// pageCountUnprotected reads a PDF with no password supplied, so an encrypted
// document returns an error. Used to prove /api/protect actually encrypted.
func pageCountUnprotected(data []byte) (int, error) {
	return api.PageCount(bytes.NewReader(data), model.NewDefaultConfiguration())
}

// requireTool skips the test when an external converter is not installed, so
// the suite still runs on a machine without ghostscript or poppler. CI installs
// both, and TestExternalToolsPresent fails there if they go missing.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed — skipping", name)
	}
}
