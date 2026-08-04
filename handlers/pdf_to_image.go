package handlers

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func PDFToImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	format := c.PostForm("format")
	if format != "jpg" && format != "png" {
		format = "png"
	}

	dpi := c.PostForm("dpi")
	if dpi != "72" && dpi != "150" && dpi != "300" {
		dpi = "150"
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

	imgDir := filepath.Join(tmpDir, "images")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		failInternal(c, "failed to create image output dir", err)
		return
	}

	// pdftoppm outputs files named: <prefix>-<page>.png / <prefix>-<page>.jpg
	outputPrefix := filepath.Join(imgDir, "page")

	args := []string{"-r", dpi}
	if format == "png" {
		args = append(args, "-png")
	} else {
		args = append(args, "-jpeg")
	}
	args = append(args, inFile, outputPrefix)

	if out, err := runTool(c.Request.Context(), "pdftoppm", args...); err != nil {
		failInternal(c, "could not convert the PDF to images", logErr(err, out))
		return
	}

	zipFile := filepath.Join(tmpDir, "images.zip")
	if err := zipDir(imgDir, zipFile); err != nil {
		failInternal(c, "failed to create zip archive", err)
		return
	}

	c.FileAttachment(zipFile, "images.zip")
}
