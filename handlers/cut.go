package handlers

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Repeated ranges can multiply a small input into a very large document.
const maxCutPages = 10000

func Cut(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}
	fields := c.PostFormArray("pages")
	if len(fields) != 1 {
		failBadRequest(c, "provide one page selection, e.g. 1,3,6-7")
		return
	}
	ranges, err := parsePageRanges(fields[0])
	if err != nil {
		failBadRequest(c, err.Error())
		return
	}
	total := 0
	selections := make([]string, 0, len(ranges))
	for _, pages := range ranges {
		total += pages.last - pages.first + 1
		if total > maxCutPages {
			failBadRequest(c, "the output is limited to 10000 pages")
			return
		}
		selections = append(selections, pages.selector())
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
	pageCount, err := api.PageCountFile(inFile)
	if err != nil {
		failBadRequest(c, "could not read the PDF page count")
		return
	}
	if err := validatePageRanges(ranges, pageCount); err != nil {
		failBadRequest(c, err.Error())
		return
	}
	if c.Request.Context().Err() != nil {
		return
	}

	outFile := filepath.Join(tmpDir, "cut.pdf")
	// Collect preserves the entered order and repeated pages, unlike set-based trimming.
	if err := api.CollectFile(inFile, outFile, selections, nil); err != nil {
		failInternal(c, "could not cut the PDF", err)
		return
	}
	c.FileAttachment(outFile, "cut.pdf")
}
