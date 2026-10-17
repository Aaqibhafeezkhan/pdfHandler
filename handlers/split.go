package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// maxSplitSpan is an upper bound on the pages-per-file value; anything larger
// is a typo rather than a real request.
const maxSplitSpan = 10000

func Split(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	// Each comma-separated entry becomes its own PDF; omitted pages are excluded.
	var ranges []pageRange
	if fields := c.PostFormArray("pages"); len(fields) > 0 {
		if len(fields) != 1 || c.PostForm("span") != "" {
			failBadRequest(c, "provide either pages or span, not both")
			return
		}
		ranges, err = parsePageRanges(fields[0])
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
		if err := validatePageRanges(ranges, pageCount); err != nil {
			failBadRequest(c, err.Error())
			return
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
			selection := pages.selector()
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
