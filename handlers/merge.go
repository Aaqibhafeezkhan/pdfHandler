package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// maxMergeFiles bounds the work a single merge request can queue up.
const maxMergeFiles = 50

func Merge(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		failUpload(c, err, "invalid form data")
		return
	}

	files := form.File["files[]"]
	if len(files) < 2 {
		failBadRequest(c, "at least 2 PDF files required")
		return
	}
	if len(files) > maxMergeFiles {
		failBadRequest(c, fmt.Sprintf("at most %d files can be merged at once", maxMergeFiles))
		return
	}

	tmpDir, err := createTempDir()
	if err != nil {
		failInternal(c, "failed to create temp dir", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	var inFiles []string
	for i, file := range files {
		path := filepath.Join(tmpDir, fmt.Sprintf("input_%03d.pdf", i))
		if err := saveUploadedPDF(c, file, path); err != nil {
			failBadRequest(c, "one of the uploaded files is not a valid PDF")
			return
		}
		inFiles = append(inFiles, path)
	}

	outFile := filepath.Join(tmpDir, "merged.pdf")
	if err := api.MergeCreateFile(inFiles, outFile, false, nil); err != nil {
		failInternal(c, "could not merge the PDFs", err)
		return
	}

	c.FileAttachment(outFile, "merged.pdf")
}
