package quality

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestExpandReportUploadRejectsUnsafeZipEntries(t *testing.T) {
	data := buildTestZip(t, []testZipFile{
		{name: "../spotbugs.xml", content: "<BugCollection></BugCollection>"},
		{name: "reports/nested.zip", content: "nested"},
		{name: "reports/spotbugs.xml", content: "<BugCollection></BugCollection>"},
		{name: "reports/readme.txt", content: "plain"},
		{name: "reports/path/../bad.xml", content: "<BugCollection></BugCollection>"},
	})

	entries, err := expandReportUpload("reports.zip", "", data)
	if err != nil {
		t.Fatalf("expandReportUpload returned error: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	if entries[0].Status != "rejected" {
		t.Fatalf("expected traversal entry rejected, got %q", entries[0].Status)
	}
	if entries[1].Status != "ignored" {
		t.Fatalf("expected nested archive ignored, got %q", entries[1].Status)
	}
	if entries[2].Status != "" || entries[2].EntryPath != "reports/spotbugs.xml" {
		t.Fatalf("expected valid report entry, got status=%q path=%q", entries[2].Status, entries[2].EntryPath)
	}
	if entries[4].Status != "rejected" {
		t.Fatalf("expected path with parent segment rejected, got %q", entries[4].Status)
	}
}

func TestParseScanReportRejectsFormatMismatch(t *testing.T) {
	trivyJSON := []byte(`{"Results":[{"Target":"nginx:latest","Vulnerabilities":[]}]}`)
	if _, _, err := parseScanReport("spotbugs", "spotbugs-xml", "xml", trivyJSON); err == nil || !strings.Contains(err.Error(), "报告格式不匹配") {
		t.Fatalf("expected format mismatch, got %v", err)
	}
	if _, _, err := parseScanReport("spotbugs", "trivy-json", "spotbugs-xml", trivyJSON); err == nil || !strings.Contains(err.Error(), "报告解析器不匹配") {
		t.Fatalf("expected parser mismatch, got %v", err)
	}
}

func TestParseTrivyJSON(t *testing.T) {
	trivyJSON := []byte(`{"Results":[{"Target":"nginx:latest","Vulnerabilities":[{"VulnerabilityID":"CVE-1","PkgName":"openssl","InstalledVersion":"1.0","FixedVersion":"1.1","Title":"漏洞","Severity":"HIGH"}]}]}`)
	summaryJSON, issuesJSON, err := parseScanReport("trivy", "trivy-json", "json", trivyJSON)
	if err != nil {
		t.Fatalf("parseScanReport returned error: %v", err)
	}
	if !bytes.Contains([]byte(summaryJSON), []byte(`"highCount":1`)) {
		t.Fatalf("expected highCount=1, got %s", summaryJSON)
	}
	if !bytes.Contains([]byte(issuesJSON), []byte(`"cveId":"CVE-1"`)) {
		t.Fatalf("expected CVE issue, got %s", issuesJSON)
	}
}

type testZipFile struct {
	name    string
	content string
}

func buildTestZip(t *testing.T, files []testZipFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, item := range files {
		file, err := writer.Create(item.name)
		if err != nil {
			t.Fatalf("create zip entry failed: %v", err)
		}
		if _, err := file.Write([]byte(item.content)); err != nil {
			t.Fatalf("write zip entry failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	return buf.Bytes()
}
