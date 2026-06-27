package qualityreport

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxReportUploadBytes    = 200 << 20
	maxReportArchiveEntries = 500
	maxReportEntryBytes     = 50 << 20
	maxReportExtractedBytes = 500 << 20
)

type Entry struct {
	EntryPath        string
	OriginalFileName string
	ContentType      string
	Size             int64
	Sha256           string
	Data             []byte
	Status           string
	Reason           string
}

func ReadLimitedReportUpload(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReportUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxReportUploadBytes {
		return nil, fmt.Errorf("上传文件超过大小限制")
	}
	return data, nil
}

func ExpandReportUpload(fileName, reportPath string, data []byte) ([]Entry, error) {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		return expandZipReportUpload(fileName, data)
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return expandTarGzReportUpload(fileName, data)
	default:
		entryPath := normalizeUploadEntryPath(firstNonEmpty(reportPath, fileName))
		if entryPath == "" {
			entryPath = filepath.Base(fileName)
		}
		return []Entry{{
			EntryPath:        entryPath,
			OriginalFileName: filepath.Base(fileName),
			ContentType:      DetectReportContentType(fileName, data),
			Size:             int64(len(data)),
			Data:             data,
		}}, nil
	}
}

func expandZipReportUpload(fileName string, data []byte) ([]Entry, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("ZIP 压缩包解析失败")
	}
	entries := make([]Entry, 0)
	var total int64
	count := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		count++
		if count > maxReportArchiveEntries {
			entries = append(entries, rejectedArchiveEntry(file.Name, "压缩包文件数量超过限制"))
			break
		}
		entry, ok := prepareArchiveEntry(file.Name)
		if !ok {
			entries = append(entries, entry)
			continue
		}
		if isNestedArchive(entry.EntryPath) {
			entry.Status = "ignored"
			entry.Reason = "默认不处理嵌套压缩包"
			entries = append(entries, entry)
			continue
		}
		rc, err := file.Open()
		if err != nil {
			entries = append(entries, rejectedArchiveEntry(file.Name, "压缩包文件读取失败"))
			continue
		}
		fileData, err := readReportArchiveFile(rc)
		_ = rc.Close()
		if err != nil {
			entry.Status = "rejected"
			entry.Reason = err.Error()
			entries = append(entries, entry)
			continue
		}
		total += int64(len(fileData))
		if total > maxReportExtractedBytes {
			entry.Status = "rejected"
			entry.Reason = "解压后总大小超过限制"
			entries = append(entries, entry)
			continue
		}
		entry.Data = fileData
		entry.Size = int64(len(fileData))
		entry.ContentType = DetectReportContentType(entry.EntryPath, fileData)
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s 中没有可处理的报告文件", fileName)
	}
	return entries, nil
}

func expandTarGzReportUpload(fileName string, data []byte) ([]Entry, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("TAR.GZ 压缩包解析失败")
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	entries := make([]Entry, 0)
	var total int64
	count := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("TAR.GZ 压缩包读取失败")
		}
		if header.FileInfo().IsDir() {
			continue
		}
		count++
		if count > maxReportArchiveEntries {
			entries = append(entries, rejectedArchiveEntry(header.Name, "压缩包文件数量超过限制"))
			break
		}
		entry, ok := prepareArchiveEntry(header.Name)
		if !ok {
			entries = append(entries, entry)
			continue
		}
		if isNestedArchive(entry.EntryPath) {
			entry.Status = "ignored"
			entry.Reason = "默认不处理嵌套压缩包"
			entries = append(entries, entry)
			continue
		}
		fileData, err := readReportArchiveFile(tarReader)
		if err != nil {
			entry.Status = "rejected"
			entry.Reason = err.Error()
			entries = append(entries, entry)
			continue
		}
		total += int64(len(fileData))
		if total > maxReportExtractedBytes {
			entry.Status = "rejected"
			entry.Reason = "解压后总大小超过限制"
			entries = append(entries, entry)
			continue
		}
		entry.Data = fileData
		entry.Size = int64(len(fileData))
		entry.ContentType = DetectReportContentType(entry.EntryPath, fileData)
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s 中没有可处理的报告文件", fileName)
	}
	return entries, nil
}

func prepareArchiveEntry(rawPath string) (Entry, bool) {
	entryPath := normalizeUploadEntryPath(rawPath)
	entry := Entry{
		EntryPath:        firstNonEmpty(entryPath, rawPath),
		OriginalFileName: filepath.Base(rawPath),
	}
	if entryPath == "" {
		entry.Status = "rejected"
		entry.Reason = "报告路径非法"
		return entry, false
	}
	entry.EntryPath = entryPath
	return entry, true
}

func rejectedArchiveEntry(entryPath, reason string) Entry {
	return Entry{
		EntryPath:        firstNonEmpty(normalizeUploadEntryPath(entryPath), entryPath),
		OriginalFileName: filepath.Base(entryPath),
		Status:           "rejected",
		Reason:           reason,
	}
}

func readReportArchiveFile(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReportEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("报告文件读取失败")
	}
	if len(data) > maxReportEntryBytes {
		return nil, fmt.Errorf("报告文件超过大小限制")
	}
	return data, nil
}

func normalizeUploadEntryPath(in string) string {
	value := strings.ReplaceAll(strings.TrimSpace(in), "\\", "/")
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return ""
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return ""
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	return cleaned
}

func isNestedArchive(fileName string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz")
}

func DetectReportContentType(fileName string, data []byte) string {
	if contentType := mime.TypeByExtension(filepath.Ext(fileName)); contentType != "" {
		return contentType
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}
