package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

const defaultPreviewLimit = 20
const maxPreviewLimit = 50

type PreviewResponse struct {
	PageCount  int      `json:"pageCount"`
	Thumbnails []string `json:"thumbnails"`
	Truncated  bool     `json:"truncated"`
}

func Preview(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	limit := defaultPreviewLimit
	if l := c.PostForm("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPreviewLimit {
		limit = maxPreviewLimit
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
		failBadRequest(c, "could not read the PDF")
		return
	}

	lastPage := pageCount
	truncated := false
	if lastPage > limit {
		lastPage = limit
		truncated = true
	}

	imgDir := filepath.Join(tmpDir, "thumbs")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		failInternal(c, "failed to create thumbnail dir", err)
		return
	}

	outputPrefix := filepath.Join(imgDir, "page")
	if out, err := runTool(c.Request.Context(), "pdftoppm",
		"-r", "96",
		"-png",
		"-f", "1",
		"-l", fmt.Sprintf("%d", lastPage),
		inFile, outputPrefix,
	); err != nil {
		failInternal(c, "thumbnail generation failed", logErr(err, out))
		return
	}

	entries, err := os.ReadDir(imgDir)
	if err != nil {
		failInternal(c, "failed to read thumbnails", err)
		return
	}

	var imgPaths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
			imgPaths = append(imgPaths, filepath.Join(imgDir, e.Name()))
		}
	}
	sort.Strings(imgPaths)

	thumbnails := make([]string, 0, len(imgPaths))
	for _, p := range imgPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		thumbnails = append(thumbnails, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(data))
	}

	c.JSON(http.StatusOK, PreviewResponse{
		PageCount:  pageCount,
		Thumbnails: thumbnails,
		Truncated:  truncated,
	})
}
