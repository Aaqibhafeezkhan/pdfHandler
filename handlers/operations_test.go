package handlers

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestMergeProducesCombinedPDF(t *testing.T) {
	r := testRouter()
	a, b := pdfBytes(t, 2), pdfBytes(t, 3)

	rec := post(t, r, "/api/merge", []upload{
		{field: "files[]", filename: "a.pdf", data: a},
		{field: "files[]", filename: "b.pdf", data: b},
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("merge = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got, want := pageCount(t, rec.Body.Bytes()), 5; got != want {
		t.Errorf("merged PDF has %d pages, want %d", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "pdf") {
		t.Errorf("Content-Type = %q, want a PDF type", ct)
	}
}

func TestSplitProducesOneFilePerPage(t *testing.T) {
	r := testRouter()

	rec := post(t, r, "/api/split",
		[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 4)}},
		map[string]string{"span": "1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("split = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	data := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("response is not a valid zip: %v", err)
	}
	if len(zr.File) != 4 {
		t.Errorf("split of a 4-page PDF produced %d files, want 4", len(zr.File))
	}
}

func TestRotatePreservesPageCount(t *testing.T) {
	r := testRouter()

	rec := post(t, r, "/api/rotate",
		[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 3)}},
		map[string]string{"degrees": "90", "pages": "1,3"})

	if rec.Code != http.StatusOK {
		t.Fatalf("rotate = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got, want := pageCount(t, rec.Body.Bytes()), 3; got != want {
		t.Errorf("rotated PDF has %d pages, want %d", got, want)
	}
}

func TestImageToPDFMakesOnePagePerImage(t *testing.T) {
	r := testRouter()
	img := pngBytes(t)

	rec := post(t, r, "/api/image-to-pdf", []upload{
		{field: "files[]", filename: "one.png", data: img},
		{field: "files[]", filename: "two.PNG", data: img},
		{field: "files[]", filename: "three.jpg", data: img},
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("image-to-pdf = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got, want := pageCount(t, rec.Body.Bytes()), 3; got != want {
		t.Errorf("PDF from 3 images has %d pages, want %d", got, want)
	}
}

// The two halves of the encryption feature only mean something together.
func TestProtectThenUnlockRoundTrips(t *testing.T) {
	r := testRouter()
	original := pdfBytes(t, 2)
	const password = "correct-horse-battery-staple"

	enc := post(t, r, "/api/protect",
		[]upload{{field: "file", filename: "in.pdf", data: original}},
		map[string]string{"password": password})
	if enc.Code != http.StatusOK {
		t.Fatalf("protect = %d, want 200 (body: %s)", enc.Code, enc.Body.String())
	}

	// An encrypted PDF must no longer be readable without the password.
	if _, err := pageCountUnprotected(enc.Body.Bytes()); err == nil {
		t.Error("protect returned a PDF that still opens without a password")
	}

	dec := post(t, r, "/api/unlock",
		[]upload{{field: "file", filename: "enc.pdf", data: enc.Body.Bytes()}},
		map[string]string{"password": password})
	if dec.Code != http.StatusOK {
		t.Fatalf("unlock = %d, want 200 (body: %s)", dec.Code, dec.Body.String())
	}
	if got, want := pageCount(t, dec.Body.Bytes()), 2; got != want {
		t.Errorf("round-tripped PDF has %d pages, want %d", got, want)
	}
}

func TestUnlockWithWrongPasswordFails(t *testing.T) {
	r := testRouter()

	enc := post(t, r, "/api/protect",
		[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 1)}},
		map[string]string{"password": "right"})
	if enc.Code != http.StatusOK {
		t.Fatalf("protect = %d, want 200", enc.Code)
	}

	dec := post(t, r, "/api/unlock",
		[]upload{{field: "file", filename: "enc.pdf", data: enc.Body.Bytes()}},
		map[string]string{"password": "wrong"})
	if dec.Code == http.StatusOK {
		t.Fatal("unlock succeeded with the wrong password")
	}
}

func TestCompressReturnsAPDF(t *testing.T) {
	requireTool(t, "gs")
	r := testRouter()

	for _, quality := range []string{"screen", "ebook", "printer", "prepress", "bogus"} {
		t.Run("quality="+quality, func(t *testing.T) {
			rec := post(t, r, "/api/compress",
				[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 2)}},
				map[string]string{"quality": quality})

			// An unknown quality falls back to the default rather than failing.
			if rec.Code != http.StatusOK {
				t.Fatalf("compress quality=%s = %d, want 200 (body: %s)",
					quality, rec.Code, rec.Body.String())
			}
			if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
				t.Error("compress did not return a PDF")
			}
		})
	}
}

func TestPDFToImageReturnsOneImagePerPage(t *testing.T) {
	requireTool(t, "pdftoppm")
	r := testRouter()

	for _, format := range []string{"png", "jpg"} {
		t.Run("format="+format, func(t *testing.T) {
			rec := post(t, r, "/api/pdf-to-image",
				[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 3)}},
				map[string]string{"format": format, "dpi": "72"})

			if rec.Code != http.StatusOK {
				t.Fatalf("pdf-to-image = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}

			data := rec.Body.Bytes()
			zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("response is not a valid zip: %v", err)
			}
			if len(zr.File) != 3 {
				t.Errorf("3-page PDF produced %d images, want 3", len(zr.File))
			}
		})
	}
}

func TestPreviewReportsPagesAndThumbnails(t *testing.T) {
	requireTool(t, "pdftoppm")
	r := testRouter()

	rec := post(t, r, "/api/preview",
		[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 3)}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got PreviewResponse
	decodeJSON(t, rec, &got)

	if got.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", got.PageCount)
	}
	if len(got.Thumbnails) != 3 {
		t.Errorf("got %d thumbnails, want 3", len(got.Thumbnails))
	}
	if got.Truncated {
		t.Error("Truncated = true for a 3-page PDF under the limit")
	}
	for i, th := range got.Thumbnails {
		if !strings.HasPrefix(th, "data:image/png;base64,") {
			t.Errorf("thumbnail %d is not a base64 PNG data URL", i)
		}
	}
}

// The limit is clamped to maxPreviewLimit so a client cannot ask the server to
// rasterise and base64 an arbitrarily large document into one response.
func TestPreviewClampsLimit(t *testing.T) {
	requireTool(t, "pdftoppm")
	r := testRouter()

	rec := post(t, r, "/api/preview",
		[]upload{{field: "file", filename: "in.pdf", data: pdfBytes(t, 3)}},
		map[string]string{"limit": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d, want 200", rec.Code)
	}

	var got PreviewResponse
	decodeJSON(t, rec, &got)

	if len(got.Thumbnails) != 1 {
		t.Errorf("limit=1 returned %d thumbnails, want 1", len(got.Thumbnails))
	}
	if got.PageCount != 3 {
		t.Errorf("PageCount = %d, want the real count 3", got.PageCount)
	}
	if !got.Truncated {
		t.Error("Truncated = false, want true when the limit cuts the document short")
	}
}
