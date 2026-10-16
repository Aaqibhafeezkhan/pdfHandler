package handlers

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Distinct page widths let the test identify the original pages, not just count them.
func splitFixture(t *testing.T) []byte {
	t.Helper()
	var pages []io.ReadSeeker
	for page := 1; page <= 8; page++ {
		imp := pdfcpu.DefaultImportConfig()
		imp.PageDim = &types.Dim{Width: float64(100 + page), Height: 200}
		imp.UserDim = true
		imp.Pos = types.Center
		var buf bytes.Buffer
		if err := api.ImportImages(nil, &buf, []io.Reader{bytes.NewReader(pngBytes(t))}, imp, nil); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, bytes.NewReader(buf.Bytes()))
	}
	var merged bytes.Buffer
	if err := api.MergeRaw(pages, &merged, false, nil); err != nil {
		t.Fatal(err)
	}
	return merged.Bytes()
}

func TestSplitCustomRanges(t *testing.T) {
	pdf := splitFixture(t)
	tests := []struct {
		name   string
		fields map[string]string
		pages  [][]int
	}{
		{"requested example", map[string]string{"pages": "1,3,6-7"}, [][]int{{1}, {3}, {6, 7}}},
		{"spaces and leading zeros", map[string]string{"pages": " 01, 3 , 06-07 "}, [][]int{{1}, {3}, {6, 7}}},
		{"input order and overlapping ranges", map[string]string{"pages": "6-8,1,6-7,1"}, [][]int{{6, 7, 8}, {1}, {6, 7}, {1}}},
		{"entire document", map[string]string{"pages": "1-8"}, [][]int{{1, 2, 3, 4, 5, 6, 7, 8}}},
		{"single page range", map[string]string{"pages": "8-8"}, [][]int{{8}}},
		{"existing chunk mode", map[string]string{"span": "3"}, [][]int{{1, 2, 3}, {4, 5, 6}, {7, 8}}},
		{"default mode", nil, [][]int{{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, testRouter(), "/api/split", []upload{{field: "file", filename: "in.pdf", data: pdf}}, tt.fields)
			if rec.Code != http.StatusOK {
				t.Fatalf("split = %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Content-Disposition"), "split.zip") {
				t.Error("missing split.zip attachment header")
			}
			zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if len(zr.File) != len(tt.pages) {
				t.Fatalf("got %d PDFs, want %d", len(zr.File), len(tt.pages))
			}
			for i, file := range zr.File {
				r, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				dims, err := api.PageDims(bytes.NewReader(data), nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(dims) != len(tt.pages[i]) {
					t.Fatalf("%s: got %d pages, want %v", file.Name, len(dims), tt.pages[i])
				}
				for j, page := range tt.pages[i] {
					if dims[j].Width != float64(100+page) {
						t.Errorf("%s page %d: width = %v, want original page %d", file.Name, j+1, dims[j].Width, page)
					}
				}
			}
		})
	}
}

func TestSplitRejectsInvalidRanges(t *testing.T) {
	pdf := pdfBytes(t, 8)
	for _, pages := range []string{
		"", " ", ",", "1,", ",1", "1,,3", "0", "0-2", "3-1", "-1", "1-", "all",
		"1.5", "1e2", "1 - 3", "1-2-3", "9", "1-9", "999999", "1000000",
		"../../etc/passwd", "1;<script>", strings.Repeat("1,", maxPageSelectors) + "1",
		strings.Repeat(" ", maxPageSelectors*14+1),
	} {
		name := pages
		if len(name) > 40 {
			name = name[:40] + "..."
		}
		t.Run(name, func(t *testing.T) {
			rec := post(t, testRouter(), "/api/split", []upload{{field: "file", filename: "in.pdf", data: pdf}}, map[string]string{"pages": pages})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
	t.Run("conflicting modes", func(t *testing.T) {
		rec := post(t, testRouter(), "/api/split", []upload{{field: "file", filename: "in.pdf", data: pdf}}, map[string]string{"pages": "1,3", "span": "2"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}
