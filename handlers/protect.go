package handlers

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// maxPasswordLen guards against absurd inputs; PDF passwords are short by spec.
const maxPasswordLen = 256

func Protect(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	password := c.PostForm("password")
	if password == "" || len(password) > maxPasswordLen {
		failBadRequest(c, "a password of up to 256 characters is required")
		return
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

	outFile := filepath.Join(tmpDir, "protected.pdf")

	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password

	if err := api.EncryptFile(inFile, outFile, conf); err != nil {
		// Logged without the password: only the pdfcpu error is passed on.
		failInternal(c, "could not encrypt the PDF", err)
		return
	}

	c.FileAttachment(outFile, "protected.pdf")
}

func Unlock(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failUpload(c, err, "PDF file required")
		return
	}

	password := c.PostForm("password")
	if password == "" || len(password) > maxPasswordLen {
		failBadRequest(c, "a password of up to 256 characters is required")
		return
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

	outFile := filepath.Join(tmpDir, "unlocked.pdf")

	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password

	if err := api.DecryptFile(inFile, outFile, conf); err != nil {
		// The pdfcpu error goes to the log only — it can quote structure from
		// the encrypted document.
		failInternal(c, "could not unlock the PDF — check the password", err)
		return
	}

	c.FileAttachment(outFile, "unlocked.pdf")
}
