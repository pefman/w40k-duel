package data

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	zipURL     = "https://github.com/BSData/wh40k-10e/archive/refs/heads/main.zip"
	outputDir  = "static/raw"
	zipFile    = "tmp/main.zip"
	extractDir = "tmp/unzip"
)

// DownloadAndExtractCats downloads the zip and extracts .cat files to outputDir
func DownloadAndExtractCats() error {
	fmt.Println("🔽 Downloading .cat archive...")

	if err := os.MkdirAll("tmp", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Download zip
	resp, err := http.Get(zipURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(zipFile)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	fmt.Println("✅ Download complete")

	// Unzip
	fmt.Println("🗂️  Extracting .cat files...")
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".cat") {
			srcFile, err := f.Open()
			if err != nil {
				return err
			}
			defer srcFile.Close()

			// Clean filename
			_, fname := filepath.Split(f.Name)
			targetPath := filepath.Join(outputDir, fname)

			dstFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, srcFile); err != nil {
				return err
			}

			fmt.Println("✅ Extracted:", fname)
		}
	}

	return nil
}
