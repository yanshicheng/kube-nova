package quality

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/yanshicheng/kube-nova/common/devops/qualityreport"
)

const (
	maxReportPreviewBytes    = 2 << 20
	reportSourceTypeCIUpload = "ci_upload"
)

type reportArchiveEntry = qualityreport.Entry

func readLimitedReportUpload(r io.Reader) ([]byte, error) {
	return qualityreport.ReadLimitedReportUpload(r)
}

func expandReportUpload(fileName, reportPath string, data []byte) ([]reportArchiveEntry, error) {
	return qualityreport.ExpandReportUpload(fileName, reportPath, data)
}

func detectReportContentType(fileName string, data []byte) string {
	return qualityreport.DetectReportContentType(fileName, data)
}

func rejectedArchiveEntry(entryPath, reason string) reportArchiveEntry {
	normalizedPath := strings.TrimSpace(strings.ReplaceAll(entryPath, "\\", "/"))
	return reportArchiveEntry{
		EntryPath:        firstNonEmptyAPI(normalizedPath, entryPath),
		OriginalFileName: filepath.Base(entryPath),
		Status:           "rejected",
		Reason:           reason,
	}
}

func firstNonEmptyAPI(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}
