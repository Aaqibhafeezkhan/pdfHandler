package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// maxSplitSpan is an upper bound on the pages-per-file value; anything larger
// is a typo rather than a real request.
const maxSplitSpan = 10000

type splitRange struct {
	first int
	last  int
}

func parseSplitRanges(pages string) ([]splitRange, error) {
	if len(pages) > maxPageSelectors*14 {
		return nil, errors.New("page selection is too long")
	}
	parts := strings.Split(pages, ",")
	if len(parts) > maxPageSelectors {
		return nil, errors.New("too many page ranges")
	}
	ranges := make([]splitRange, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !pageSelector.MatchString(part) {
			return nil, errors.New("invalid page selection: use numbers or ranges, e.g. 1,3,6-7")
		}
		bounds := strings.Split(part, "-")
		first, _ := strconv.Atoi(bounds[0]) // The pattern bounds integers to six digits.
		last := first
		if len(bounds) == 2 {
			last, _ = strconv.Atoi(bounds[1])
		}
		if first < 1 || last < first {
			return nil, errors.New("page ranges must start at 1 or greater and end at or after their start")
		}
		ranges = append(ranges, splitRange{first: first, last: last})
	}
	return ranges, nil
}

func Split(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	// Each comma-separated entry becomes its own PDF; omitted pages are excluded.
	var ranges []splitRange
	if fields := c.PostFormArray("pages"); len(fields) > 0 {
		if len(fields) != 1 || c.PostForm("span") != "" {
			failBadRequest(c, "provide either pages or span, not both")
			return
		}
		ranges, err = parseSplitRanges(fields[0])
		if err != nil {
			failBadRequest(c, err.Error())
			return
		}
	}

	// span = number of pages per output file; 1 = one page per file.
	span := 1
	if s := c.PostForm("span"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 || n > maxSplitSpan {
			failBadRequest(c, "span must be a positive number of pages")
			return
		}
		span = n
	}

	tmpDir, err := createTempDir()
	if err != nil {
		failInternal(c, "failed to create temp dir", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	inFile := filepath.Join(tmpDir, "input.pdf")
	if err := saveUploadedPDF(c, file, inFile); err != nil {
		failBadRequest(c, "the uploaded file is not a valid PDF")
		return
	}
	if len(ranges) > 0 {
		pageCount, err := api.PageCountFile(inFile)
		if err != nil {
			failBadRequest(c, "could not read the PDF page count")
			return
		}
		for _, pages := range ranges {
			if pages.last > pageCount {
				failBadRequest(c, fmt.Sprintf("page selection exceeds the PDF's %d pages", pageCount))
				return
			}
		}
	}

	outDir := filepath.Join(tmpDir, "split")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		failInternal(c, "failed to create output dir", err)
		return
	}

	if len(ranges) == 0 {
		if err := api.SplitFile(inFile, outDir, span, nil); err != nil {
			failInternal(c, "could not split the PDF", err)
			return
		}
	} else {
		for i, pages := range ranges {
			if c.Request.Context().Err() != nil {
				return
			}
			selection := strconv.Itoa(pages.first)
			if pages.last != pages.first {
				selection += "-" + strconv.Itoa(pages.last)
			}
			// Use only validated page numbers and an index; duplicate ranges stay distinct.
			outFile := filepath.Join(outDir, fmt.Sprintf("%04d_pages_%s.pdf", i+1, selection))
			if err := api.TrimFile(inFile, outFile, []string{selection}, nil); err != nil {
				failInternal(c, "could not split the PDF", err)
				return
			}
		}
	}

	zipFile := filepath.Join(tmpDir, "split.zip")
	if err := zipDir(outDir, zipFile); err != nil {
		failInternal(c, "failed to create zip archive", err)
		return
	}

	c.FileAttachment(zipFile, "split.zip")
}
