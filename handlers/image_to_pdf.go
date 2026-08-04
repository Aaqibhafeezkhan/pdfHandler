package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// maxImageFiles bounds the work a single conversion request can queue up.
const maxImageFiles = 100

func ImageToPDF(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		failUpload(c, err, "invalid form data")
		return
	}

	files := form.File["files[]"]
	if len(files) == 0 {
		failBadRequest(c, "at least one image file required")
		return
	}
	if len(files) > maxImageFiles {
		failBadRequest(c, fmt.Sprintf("at most %d images can be converted at once", maxImageFiles))
		return
	}

	tmpDir, err := createTempDir()
	if err != nil {
		failInternal(c, "failed to create temp dir", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	var imgFiles []string
	for i, file := range files {
		// The extension comes from the client-supplied filename, so it is
		// sanitised and allow-listed before it is ever joined onto a path.
		ext, err := safeImageExt(file.Filename)
		if err != nil {
			failBadRequest(c, "unsupported image type: use JPG, PNG, TIFF or WebP")
			return
		}
		path := filepath.Join(tmpDir, fmt.Sprintf("img_%03d%s", i, ext))
		if err := c.SaveUploadedFile(file, path); err != nil {
			failInternal(c, "failed to save uploaded file", err)
			return
		}
		imgFiles = append(imgFiles, path)
	}

	outFile := filepath.Join(tmpDir, "output.pdf")
	// nil import config: each image is placed on its own page at original size
	if err := api.ImportImagesFile(imgFiles, outFile, nil, nil); err != nil {
		failInternal(c, "could not convert the images", err)
		return
	}

	c.FileAttachment(outFile, "output.pdf")
}
