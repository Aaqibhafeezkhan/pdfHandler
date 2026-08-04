package handlers

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// validQualities maps accepted quality values to ghostscript PDFSETTINGS.
// screen   ~72 dpi  — smallest file, lowest quality
// ebook    ~150 dpi — good balance (default)
// printer  ~300 dpi — high quality
// prepress ~300 dpi — highest quality, color-preserving
var validQualities = map[string]bool{
	"screen":   true,
	"ebook":    true,
	"printer":  true,
	"prepress": true,
}

func Compress(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	quality := c.PostForm("quality")
	if !validQualities[quality] {
		quality = "ebook"
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

	outFile := filepath.Join(tmpDir, "compressed.pdf")

	// -dSAFER sandboxes the PostScript interpreter: no file access outside the
	// documents given on the command line, no %pipe% device. It is the default
	// from ghostscript 9.50 on, but the input here is untrusted so it is set
	// explicitly rather than inherited from whatever version is installed.
	if out, err := runTool(c.Request.Context(), "gs",
		"-sDEVICE=pdfwrite",
		"-dSAFER",
		"-dCompatibilityLevel=1.4",
		"-dPDFSETTINGS=/"+quality,
		"-dNOPAUSE",
		"-dQUIET",
		"-dBATCH",
		"-sOutputFile="+outFile,
		inFile,
	); err != nil {
		failInternal(c, "could not compress the PDF", logErr(err, out))
		return
	}

	c.FileAttachment(outFile, "compressed.pdf")
}
