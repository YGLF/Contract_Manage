package handlers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZipFile(t *testing.T, entries map[string]string) string {
	t.Helper()

	tempFile := filepath.Join(t.TempDir(), "test.docx")
	file, err := os.Create(tempFile)
	if err != nil {
		t.Fatalf("failed to create temp docx: %v", err)
	}

	zipWriter := zip.NewWriter(file)
	for name, content := range entries {
		entryWriter, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}

		if _, err := entryWriter.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("failed to close temp docx file: %v", err)
	}

	return tempFile
}

func TestAllowedDocumentExtensionsBlockHTML(t *testing.T) {
	if isAllowedDocumentExtension(".html") {
		t.Fatal("expected .html upload to be blocked")
	}
	if isAllowedDocumentExtension(".htm") {
		t.Fatal("expected .htm upload to be blocked")
	}
	if !isAllowedDocumentExtension(".pdf") || !isAllowedDocumentExtension(".docx") || !isAllowedDocumentExtension(".txt") {
		t.Fatal("expected safe upload extensions to remain allowed")
	}
}

func TestPreviewableDocumentExtensionsBlockHTML(t *testing.T) {
	if isPreviewableDocumentExtension(".html") {
		t.Fatal("expected .html preview to be blocked")
	}
	if isPreviewableDocumentExtension(".htm") {
		t.Fatal("expected .htm preview to be blocked")
	}
	if !isPreviewableDocumentExtension(".pdf") || !isPreviewableDocumentExtension(".jpg") || !isPreviewableDocumentExtension(".txt") {
		t.Fatal("expected safe preview extensions to remain allowed")
	}
}

func TestExtractTextFromDocxReturnsTextWithinLimits(t *testing.T) {
	docxPath := writeZipFile(t, map[string]string{
		"word/document.xml": `<w:document><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p><w:p><w:r><w:t>World</w:t></w:r></w:p></w:body></w:document>`,
	})

	text, err := extractTextFromDocx(docxPath)
	if err != nil {
		t.Fatalf("expected valid docx to be parsed, got error: %v", err)
	}

	if !strings.Contains(text, "Hello") || !strings.Contains(text, "World") {
		t.Fatalf("expected extracted text to contain document content, got %q", text)
	}
}

func TestExtractTextFromDocxRejectsOversizedEntry(t *testing.T) {
	oversizedEntry := `<w:document><w:body><w:p><w:r><w:t>` + strings.Repeat("A", maxDocxEntryBytes+1) + `</w:t></w:r></w:p></w:body></w:document>`
	docxPath := writeZipFile(t, map[string]string{
		"word/document.xml": oversizedEntry,
	})

	_, err := extractTextFromDocx(docxPath)
	if err == nil {
		t.Fatal("expected oversized docx entry to fail")
	}

	if !strings.Contains(err.Error(), "超出限制") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestExtractTextFromDocxRejectsOversizedArchiveContent(t *testing.T) {
	docxPath := writeZipFile(t, map[string]string{
		"word/document.xml": `<w:document><w:body><w:p><w:r><w:t>ok</w:t></w:r></w:p></w:body></w:document>`,
		"word/media/blob1.bin": strings.Repeat("B", maxDocxEntryBytes),
		"word/media/blob2.bin": strings.Repeat("C", maxDocxEntryBytes),
		"word/media/blob3.bin": strings.Repeat("D", maxDocxEntryBytes),
		"word/media/blob4.bin": strings.Repeat("E", maxDocxEntryBytes),
	})

	_, err := extractTextFromDocx(docxPath)
	if err == nil {
		t.Fatal("expected oversized docx archive to fail")
	}

	if !strings.Contains(err.Error(), "超出限制") {
		t.Fatalf("expected archive size limit error, got %v", err)
	}
}
