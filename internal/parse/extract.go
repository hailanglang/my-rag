package parse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ErrNoExtractablePDFText is returned when a PDF has no text layer (OCR is not supported).
var ErrNoExtractablePDFText = errors.New("pdf: no extractable text (OCR not supported)")

// Extract returns UTF-8 plain text from a file path based on extension.
// Supported: .txt, .md (UTF-8), .pdf (text layer only; no OCR).
func Extract(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case ".pdf":
		return extractPDF(path)
	default:
		return "", fmt.Errorf("unsupported file type %q", ext)
	}
}

func extractPDF(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	r, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", fmt.Errorf("pdf: open: %w", err)
	}

	pr, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf: plain text: %w", err)
	}
	plain, err := io.ReadAll(pr)
	if err != nil {
		return "", fmt.Errorf("pdf: read: %w", err)
	}
	out := strings.TrimSpace(string(plain))
	if out == "" {
		return "", ErrNoExtractablePDFText
	}
	return out, nil
}
