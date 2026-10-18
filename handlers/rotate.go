package handlers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func Rotate(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	// pdfcpu only understands quarter turns; anything else is rejected rather
	// than silently rounded.
	degrees := 90
	if d := c.PostForm("degrees"); d != "" {
		n, err := strconv.Atoi(d)
		if err != nil || n%90 != 0 || n == 0 {
			failBadRequest(c, "degrees must be a non-zero multiple of 90")
			return
		}
		degrees = n
	}

	// pages: "all" (default) or comma-separated page numbers/ranges e.g. "1,3,5-7"
	var selectedPages []string
	if pages := c.PostForm("pages"); pages != "" && pages != "all" {
		for _, p := range strings.Split(pages, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !pageSelector.MatchString(p) {
				failBadRequest(c, "invalid page selection: use numbers or ranges, e.g. 1,3,5-7")
				return
			}
			selectedPages = append(selectedPages, p)
		}
		if len(selectedPages) > maxPageSelectors {
			failBadRequest(c, "too many page selectors")
			return
		}
	}
	// nil selectedPages means pdfcpu rotates all pages

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

	outFile := filepath.Join(tmpDir, "rotated.pdf")
	if err := api.RotateFile(inFile, outFile, degrees, selectedPages, nil); err != nil {
		failInternal(c, "could not rotate the PDF", err)
		return
	}

	c.FileAttachment(outFile, "rotated.pdf")
}
