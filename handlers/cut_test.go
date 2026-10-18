package handlers

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestCutKeepsRequestedPagesInOnePDF(t *testing.T) {
	pdf := splitFixture(t)
	for _, tt := range []struct {
		selection string
		pages     []int
	}{
		{"1,3,6-7", []int{1, 3, 6, 7}},
		{" 01, 3 , 06-07 ", []int{1, 3, 6, 7}},
		{"6-7,1,3", []int{6, 7, 1, 3}},
		{"1,1,6-7", []int{1, 1, 6, 7}},
		{"8-8", []int{8}},
		{"1-8", []int{1, 2, 3, 4, 5, 6, 7, 8}},
	} {
		t.Run(tt.selection, func(t *testing.T) {
			rec := post(t, testRouter(), "/api/cut", []upload{{field: "file", filename: "in.pdf", data: pdf}}, map[string]string{"pages": tt.selection})
			if rec.Code != http.StatusOK {
				t.Fatalf("cut = %d: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "pdf") || !strings.Contains(rec.Header().Get("Content-Disposition"), "cut.pdf") {
				t.Fatal("response is not a cut.pdf attachment")
			}
			dims, err := api.PageDims(bytes.NewReader(rec.Body.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(dims) != len(tt.pages) {
				t.Fatalf("got %d pages, want %v", len(dims), tt.pages)
			}
			for i, page := range tt.pages {
				if dims[i].Width != float64(100+page) {
					t.Errorf("output page %d: width = %v, want original page %d", i+1, dims[i].Width, page)
				}
			}
		})
	}
}

func TestCutRejectsInvalidSelection(t *testing.T) {
	pdf := pdfBytes(t, 8)
	for _, pages := range []string{"", " ", ",", "1,,3", "0", "-1", "3-1", "1-", "1-2-3", "9", "1-9", "1.5", "1e2", "all", "<script>", "../../secret", "1-10001", strings.Repeat("1,", maxPageSelectors) + "1", strings.Repeat(" ", maxPageSelectors*14+1)} {
		name := pages
		if len(name) > 40 {
			name = name[:40] + "..."
		}
		t.Run(name, func(t *testing.T) {
			rec := post(t, testRouter(), "/api/cut", []upload{{field: "file", filename: "in.pdf", data: pdf}}, map[string]string{"pages": pages})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
	for _, tt := range []struct {
		name   string
		data   []byte
		fields map[string]string
	}{
		{"missing selection", pdf, nil},
		{"non-PDF upload", []byte("GIF89a"), map[string]string{"pages": "1"}},
		{"broken PDF", []byte("%PDF-1.4\nbroken"), map[string]string{"pages": "1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, testRouter(), "/api/cut", []upload{{field: "file", filename: "in.pdf", data: tt.data}}, tt.fields)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "input.pdf") || strings.Contains(rec.Body.String(), "pdfhandler-") {
				t.Error("error response leaks internal paths")
			}
		})
	}
	if rec := post(t, testRouter(), "/api/cut", nil, map[string]string{"pages": "1"}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing file = %d, want 400", rec.Code)
	}
}
