package documentimport

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const docxMediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

func validateFiles(files []SourceFile) ([]SourceFile, string, error) {
	if len(files) == 0 || len(files) > MaxFiles {
		return nil, "", issue("files", "file_count")
	}
	total := 0
	seen := map[string]bool{}
	hashes := []string{}
	for index := range files {
		file := &files[index]
		file.Name = path.Base(strings.ReplaceAll(strings.TrimSpace(file.Name), "\\", "/"))
		if len(file.Name) == 0 || len(file.Name) > 240 || file.Name == "." || strings.IndexFunc(file.Name, unicode.IsControl) >= 0 {
			return nil, "", issue("files", "invalid_filename")
		}
		file.Size = len(file.Content)
		total += file.Size
		if file.Size == 0 || file.Size > MaxFileBytes || total > MaxTotalBytes {
			return nil, "", issue("files", "file_size")
		}
		detected := http.DetectContentType(file.Content)
		ext := strings.ToLower(path.Ext(file.Name))
		switch {
		case detected == "image/png" || detected == "image/jpeg" || detected == "image/webp":
			file.MediaType = detected
			config, _, err := image.DecodeConfig(bytes.NewReader(file.Content))
			if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 20000 || config.Height > 20000 || int64(config.Width)*int64(config.Height) > 40000000 {
				return nil, "", issue("files", "invalid_image")
			}
		case bytes.HasPrefix(file.Content, []byte("%PDF-")):
			file.MediaType = "application/pdf"
		case ext == ".docx":
			file.MediaType = docxMediaType
			if _, err := extractDocx(file.Content); err != nil {
				return nil, "", err
			}
		case ext == ".csv" || ext == ".txt":
			if !utf8.Valid(file.Content) || bytes.Contains(file.Content, []byte{0}) || file.Size > MaxTextBytes {
				return nil, "", issue("files", "invalid_text")
			}
			file.MediaType = "text/plain"
			if ext == ".csv" {
				file.MediaType = "text/csv"
				if _, err := readCSV(file.Content); err != nil {
					return nil, "", err
				}
			}
		default:
			return nil, "", issue("files", "unsupported_file")
		}
		file.ID = uuid.New()
		file.SHA256 = fmt.Sprintf("%x", sha256.Sum256(file.Content))
		if seen[file.SHA256] {
			return nil, "", issue("files", "duplicate_file")
		}
		seen[file.SHA256] = true
		hashes = append(hashes, file.SHA256)
	}
	sort.Strings(hashes)
	return files, fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(hashes, ":")))), nil
}

func extractDocx(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil || len(reader.File) > 2000 {
		return "", issue("files", "invalid_document")
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		if file.UncompressedSize64 > 2*1024*1024 {
			return "", issue("files", "text_limit")
		}
		body, err := file.Open()
		if err != nil {
			return "", issue("files", "invalid_document")
		}
		defer body.Close()
		decoder := xml.NewDecoder(io.LimitReader(body, 2*1024*1024+1))
		var text strings.Builder
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", issue("files", "invalid_document")
			}
			switch node := token.(type) {
			case xml.StartElement:
				if node.Name.Local == "t" {
					var value string
					if decoder.DecodeElement(&value, &node) != nil {
						return "", issue("files", "invalid_document")
					}
					text.WriteString(value)
				} else if node.Name.Local == "tab" {
					text.WriteString("\t")
				}
			case xml.EndElement:
				if node.Name.Local == "p" || node.Name.Local == "tr" {
					text.WriteString("\n")
				}
				if node.Name.Local == "tc" {
					text.WriteString("\t")
				}
			}
			if text.Len() > MaxTextBytes {
				return "", issue("files", "text_limit")
			}
		}
		if strings.TrimSpace(text.String()) == "" {
			return "", issue("files", "empty_document")
		}
		return text.String(), nil
	}
	return "", issue("files", "invalid_document")
}

func readCSV(content []byte) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(content), "\uFEFF")))
	rows := [][]string{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) > 32 || len(rows) > MaxLines {
			return nil, issue("files", "invalid_csv")
		}
		rows = append(rows, row)
	}
	if len(rows) < 2 {
		return nil, issue("files", "invalid_csv")
	}
	return rows, nil
}
