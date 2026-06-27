package qualityreport

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type parsedReportSummary struct {
	IssueCount         int64   `json:"issueCount,omitempty"`
	CriticalCount      int64   `json:"criticalCount,omitempty"`
	HighCount          int64   `json:"highCount,omitempty"`
	MediumCount        int64   `json:"mediumCount,omitempty"`
	LowCount           int64   `json:"lowCount,omitempty"`
	InfoCount          int64   `json:"infoCount,omitempty"`
	BugCount           int64   `json:"bugCount,omitempty"`
	VulnerabilityCount int64   `json:"vulnerabilityCount,omitempty"`
	CodeSmellCount     int64   `json:"codeSmellCount,omitempty"`
	Coverage           float64 `json:"coverage,omitempty"`
}

type parsedIssuePayload struct {
	IssueType        string `json:"issueType"`
	Severity         string `json:"severity"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Message          string `json:"message"`
	FilePath         string `json:"filePath"`
	Line             int64  `json:"line"`
	EndLine          int64  `json:"endLine"`
	Component        string `json:"component"`
	PackageName      string `json:"packageName"`
	InstalledVersion string `json:"installedVersion"`
	FixedVersion     string `json:"fixedVersion"`
	CveID            string `json:"cveId"`
	RuleID           string `json:"ruleId"`
	RuleName         string `json:"ruleName"`
	ImageRef         string `json:"imageRef"`
	ImageDigest      string `json:"imageDigest"`
	RawJSON          string `json:"rawJson"`
}

func ParseScanReport(tool, parser, reportFormat string, data []byte) (string, string, error) {
	parser = normalizeParserCode(parser, reportFormat, tool)
	if err := validateParserContract(tool, parser, reportFormat); err != nil {
		return "", "", err
	}
	if err := validateReportContentFamily(parser, data); err != nil {
		return "", "", err
	}
	var (
		summary parsedReportSummary
		issues  []parsedIssuePayload
		err     error
	)
	switch parser {
	case "trivy-json":
		summary, issues, err = parseTrivyJSON(data)
	case "spotbugs-xml":
		summary, issues, err = parseSpotBugsXML(data)
	case "sonarqube-api", "sonarqube-json":
		summary, issues, err = parseSonarQubeJSON(data)
	case "opengrep-json", "semgrep-json":
		summary, issues, err = parseOpenGrepJSON(data)
	case "opengrep-sarif", "semgrep-sarif":
		summary, issues, err = parseSARIF(data)
	case "megalinter-json":
		summary, issues, err = parseGenericJSONIssues(data)
	case "trivy-html", "spotbugs-html", "megalinter-html", "opengrep-html":
		return "", "", fmt.Errorf("HTML 解析器需要真实报告样例后才能启用")
	default:
		return "", "", fmt.Errorf("报告解析器暂不支持")
	}
	if err != nil {
		return "", "", err
	}
	summary = enrichSummary(summary, issues)
	summaryBytes, _ := json.Marshal(summary)
	issueBytes, _ := json.Marshal(issues)
	return string(summaryBytes), string(issueBytes), nil
}

func validateParserContract(tool, parser, reportFormat string) error {
	tool = strings.ToLower(strings.TrimSpace(tool))
	parser = strings.ToLower(strings.TrimSpace(parser))
	format := strings.ToLower(strings.TrimSpace(reportFormat))
	if parser == "" {
		return fmt.Errorf("报告解析器暂不支持")
	}
	switch parser {
	case "trivy-json":
		return requireParserContract(tool, format, []string{"trivy"}, []string{"json", "trivy-json"})
	case "spotbugs-xml":
		return requireParserContract(tool, format, []string{"spotbugs"}, []string{"xml", "spotbugs-xml"})
	case "sonarqube-api":
		return requireParserContract(tool, format, []string{"sonarqube"}, []string{"api", "json", "sonarqube-api"})
	case "sonarqube-json":
		return requireParserContract(tool, format, []string{"sonarqube"}, []string{"json", "sonarqube-json"})
	case "opengrep-json", "semgrep-json":
		return requireParserContract(tool, format, []string{"opengrep", "semgrep"}, []string{"json", "opengrep-json", "semgrep-json"})
	case "opengrep-sarif", "semgrep-sarif":
		return requireParserContract(tool, format, []string{"opengrep", "semgrep"}, []string{"sarif", "opengrep-sarif", "semgrep-sarif"})
	case "megalinter-json":
		return requireParserContract(tool, format, []string{"megalinter"}, []string{"json", "megalinter-json"})
	case "trivy-html":
		return requireParserContract(tool, format, []string{"trivy"}, []string{"html", "trivy-html"})
	case "spotbugs-html":
		return requireParserContract(tool, format, []string{"spotbugs"}, []string{"html", "spotbugs-html"})
	case "megalinter-html":
		return requireParserContract(tool, format, []string{"megalinter"}, []string{"html", "megalinter-html"})
	case "opengrep-html":
		return requireParserContract(tool, format, []string{"opengrep", "semgrep"}, []string{"html", "opengrep-html", "semgrep-html"})
	default:
		return fmt.Errorf("报告解析器暂不支持")
	}
}

func requireParserContract(tool, format string, tools, formats []string) error {
	if tool != "" && !containsNormalized(tools, tool) {
		return fmt.Errorf("报告解析器不匹配")
	}
	if format != "" && !containsNormalized(formats, format) {
		return fmt.Errorf("报告格式不匹配")
	}
	return nil
}

func containsNormalized(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func validateReportContentFamily(parser string, data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("报告内容为空")
	}
	if parser == "spotbugs-xml" {
		if trimmed[0] != '<' {
			return fmt.Errorf("报告格式不匹配")
		}
		return nil
	}
	if strings.HasSuffix(parser, "-html") {
		return nil
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return fmt.Errorf("报告格式不匹配")
	}
	return nil
}

func normalizeParserCode(parser, reportFormat, tool string) string {
	value := strings.ToLower(strings.TrimSpace(parser))
	if value != "" {
		return value
	}
	format := strings.ToLower(strings.TrimSpace(reportFormat))
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool == "trivy" && format == "json" {
		return "trivy-json"
	}
	if tool == "spotbugs" && strings.Contains(format, "xml") {
		return "spotbugs-xml"
	}
	if tool == "sonarqube" {
		return "sonarqube-json"
	}
	if tool == "opengrep" {
		if format == "sarif" {
			return "opengrep-sarif"
		}
		return "opengrep-json"
	}
	if tool == "megalinter" {
		return "megalinter-json"
	}
	return value
}

func parseSARIF(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var report struct {
		Runs []struct {
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int64 `json:"startLine"`
							EndLine   int64 `json:"endLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("SARIF 报告解析失败")
	}
	if report.Runs == nil {
		return parsedReportSummary{}, nil, fmt.Errorf("SARIF 报告格式不匹配")
	}
	issues := make([]parsedIssuePayload, 0)
	for _, run := range report.Runs {
		for _, result := range run.Results {
			filePath := ""
			var line int64
			var endLine int64
			if len(result.Locations) > 0 {
				location := result.Locations[0].PhysicalLocation
				filePath = location.ArtifactLocation.URI
				line = location.Region.StartLine
				endLine = location.Region.EndLine
			}
			title := result.Message.Text
			if title == "" {
				title = result.RuleID
			}
			issues = append(issues, parsedIssuePayload{
				IssueType:   "code_smell",
				Severity:    sarifSeverity(result.Level),
				Title:       title,
				Description: title,
				Message:     title,
				FilePath:    filePath,
				Line:        line,
				EndLine:     endLine,
				RuleID:      result.RuleID,
				RawJSON:     mustJSON(result),
			})
		}
	}
	return parsedReportSummary{}, issues, nil
}

func parseTrivyJSON(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var report struct {
		Results []struct {
			Target          string `json:"Target"`
			Vulnerabilities []struct {
				VulnerabilityID  string `json:"VulnerabilityID"`
				PkgName          string `json:"PkgName"`
				InstalledVersion string `json:"InstalledVersion"`
				FixedVersion     string `json:"FixedVersion"`
				Title            string `json:"Title"`
				Description      string `json:"Description"`
				Severity         string `json:"Severity"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("Trivy JSON 解析失败")
	}
	if len(report.Results) == 0 {
		return parsedReportSummary{}, nil, fmt.Errorf("Trivy JSON 格式不匹配")
	}
	issues := make([]parsedIssuePayload, 0)
	for _, result := range report.Results {
		for _, vuln := range result.Vulnerabilities {
			title := vuln.Title
			if title == "" {
				title = vuln.VulnerabilityID
			}
			issues = append(issues, parsedIssuePayload{
				IssueType:        "vulnerability",
				Severity:         normalizeSeverity(vuln.Severity),
				Title:            title,
				Description:      vuln.Description,
				Message:          title,
				PackageName:      vuln.PkgName,
				InstalledVersion: vuln.InstalledVersion,
				FixedVersion:     vuln.FixedVersion,
				CveID:            vuln.VulnerabilityID,
				ImageRef:         result.Target,
				RawJSON:          mustJSON(vuln),
			})
		}
	}
	return parsedReportSummary{}, issues, nil
}

func parseSpotBugsXML(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var report struct {
		XMLName      xml.Name `xml:"BugCollection"`
		BugInstances []struct {
			Type         string `xml:"type,attr"`
			Priority     string `xml:"priority,attr"`
			Category     string `xml:"category,attr"`
			ShortMessage string `xml:"ShortMessage"`
			LongMessage  string `xml:"LongMessage"`
			SourceLine   struct {
				SourcePath string `xml:"sourcepath,attr"`
				SourceFile string `xml:"sourcefile,attr"`
				Start      string `xml:"start,attr"`
				End        string `xml:"end,attr"`
			} `xml:"SourceLine"`
			Class struct {
				ClassName  string `xml:"classname,attr"`
				SourceLine struct {
					SourcePath string `xml:"sourcepath,attr"`
					SourceFile string `xml:"sourcefile,attr"`
					Start      string `xml:"start,attr"`
					End        string `xml:"end,attr"`
				} `xml:"SourceLine"`
			} `xml:"Class"`
		} `xml:"BugInstance"`
	}
	if err := xml.Unmarshal(data, &report); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("SpotBugs XML 解析失败")
	}
	if report.XMLName.Local != "BugCollection" {
		return parsedReportSummary{}, nil, fmt.Errorf("SpotBugs XML 格式不匹配")
	}
	issues := make([]parsedIssuePayload, 0, len(report.BugInstances))
	for _, bug := range report.BugInstances {
		sourcePath := bug.SourceLine.SourcePath
		start := bug.SourceLine.Start
		end := bug.SourceLine.End
		if sourcePath == "" {
			sourcePath = bug.Class.SourceLine.SourcePath
			start = bug.Class.SourceLine.Start
			end = bug.Class.SourceLine.End
		}
		title := bug.ShortMessage
		if title == "" {
			title = bug.Type
		}
		issues = append(issues, parsedIssuePayload{
			IssueType:   "bug",
			Severity:    spotBugsSeverity(bug.Priority),
			Title:       title,
			Description: bug.LongMessage,
			Message:     title,
			FilePath:    sourcePath,
			Line:        parseInt64(start),
			EndLine:     parseInt64(end),
			Component:   bug.Class.ClassName,
			RuleID:      bug.Type,
			RuleName:    bug.Category,
			RawJSON:     mustJSON(bug),
		})
	}
	return parsedReportSummary{}, issues, nil
}

func parseSonarQubeJSON(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var report struct {
		Issues []struct {
			Key       string `json:"key"`
			Rule      string `json:"rule"`
			Severity  string `json:"severity"`
			Type      string `json:"type"`
			Message   string `json:"message"`
			Component string `json:"component"`
			Line      int64  `json:"line"`
			TextRange struct {
				StartLine int64 `json:"startLine"`
				EndLine   int64 `json:"endLine"`
			} `json:"textRange"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("SonarQube JSON 解析失败")
	}
	if report.Issues == nil {
		return parsedReportSummary{}, nil, fmt.Errorf("SonarQube JSON 格式不匹配")
	}
	issues := make([]parsedIssuePayload, 0, len(report.Issues))
	for _, item := range report.Issues {
		line := item.Line
		if line == 0 {
			line = item.TextRange.StartLine
		}
		issues = append(issues, parsedIssuePayload{
			IssueType:   normalizeIssueType(item.Type),
			Severity:    sonarSeverity(item.Severity),
			Title:       item.Message,
			Description: item.Message,
			Message:     item.Message,
			FilePath:    item.Component,
			Line:        line,
			EndLine:     item.TextRange.EndLine,
			Component:   item.Component,
			RuleID:      item.Rule,
			RawJSON:     mustJSON(item),
		})
	}
	return parsedReportSummary{}, issues, nil
}

func parseOpenGrepJSON(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var report struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int64 `json:"line"`
			} `json:"start"`
			End struct {
				Line int64 `json:"line"`
			} `json:"end"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("OpenGrep JSON 解析失败")
	}
	if report.Results == nil {
		return parsedReportSummary{}, nil, fmt.Errorf("OpenGrep JSON 格式不匹配")
	}
	issues := make([]parsedIssuePayload, 0, len(report.Results))
	for _, item := range report.Results {
		issues = append(issues, parsedIssuePayload{
			IssueType:   "code_smell",
			Severity:    openGrepSeverity(item.Extra.Severity),
			Title:       item.Extra.Message,
			Description: item.Extra.Message,
			Message:     item.Extra.Message,
			FilePath:    item.Path,
			Line:        item.Start.Line,
			EndLine:     item.End.Line,
			RuleID:      item.CheckID,
			RawJSON:     mustJSON(item),
		})
	}
	return parsedReportSummary{}, issues, nil
}

func parseGenericJSONIssues(data []byte) (parsedReportSummary, []parsedIssuePayload, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return parsedReportSummary{}, nil, fmt.Errorf("JSON 报告解析失败")
	}
	issues := make([]parsedIssuePayload, 0)
	collectGenericIssues(root, &issues)
	if len(issues) == 0 {
		return parsedReportSummary{}, nil, fmt.Errorf("JSON 报告未识别到问题列表")
	}
	return parsedReportSummary{}, issues, nil
}

func collectGenericIssues(value any, issues *[]parsedIssuePayload) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			collectGenericIssues(item, issues)
		}
	case map[string]any:
		message := firstString(v, "message", "msg", "title", "description")
		path := firstString(v, "path", "file", "filePath", "filename")
		if message != "" && path != "" {
			*issues = append(*issues, parsedIssuePayload{
				IssueType:   normalizeIssueType(firstString(v, "type", "issueType", "category")),
				Severity:    normalizeSeverity(firstString(v, "severity", "level")),
				Title:       message,
				Description: message,
				Message:     message,
				FilePath:    path,
				Line:        numberField(v, "line"),
				RuleID:      firstString(v, "rule", "ruleId", "rule_id", "code"),
				RawJSON:     mustJSON(v),
			})
			return
		}
		for _, item := range v {
			collectGenericIssues(item, issues)
		}
	}
}

func enrichSummary(summary parsedReportSummary, issues []parsedIssuePayload) parsedReportSummary {
	summary.IssueCount = int64(len(issues))
	for _, item := range issues {
		switch normalizeSeverity(item.Severity) {
		case "critical":
			summary.CriticalCount++
		case "high":
			summary.HighCount++
		case "medium":
			summary.MediumCount++
		case "low":
			summary.LowCount++
		default:
			summary.InfoCount++
		}
		switch normalizeIssueType(item.IssueType) {
		case "bug":
			summary.BugCount++
		case "vulnerability":
			summary.VulnerabilityCount++
		case "code_smell":
			summary.CodeSmellCount++
		}
	}
	return summary
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "blocker", "error":
		return "critical"
	case "high", "major":
		return "high"
	case "medium", "warning", "minor":
		return "medium"
	case "low":
		return "low"
	default:
		return "info"
	}
}

func spotBugsSeverity(priority string) string {
	switch strings.TrimSpace(priority) {
	case "1", "0":
		return "high"
	case "2":
		return "medium"
	case "3":
		return "low"
	default:
		return "info"
	}
}

func sonarSeverity(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BLOCKER", "CRITICAL":
		return "critical"
	case "MAJOR":
		return "high"
	case "MINOR":
		return "medium"
	case "INFO":
		return "low"
	default:
		return normalizeSeverity(value)
	}
}

func openGrepSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "info", "note":
		return "low"
	default:
		return normalizeSeverity(value)
	}
}

func sarifSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "note":
		return "low"
	default:
		return normalizeSeverity(value)
	}
}

func normalizeIssueType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BUG":
		return "bug"
	case "VULNERABILITY", "SECURITY", "CVE":
		return "vulnerability"
	case "CODE_SMELL", "CODE SMELL", "STYLE":
		return "code_smell"
	default:
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return "code_smell"
		}
		return value
	}
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func numberField(data map[string]any, key string) int64 {
	value, ok := data[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		return parseInt64(v)
	default:
		return 0
	}
}

func parseInt64(value string) int64 {
	i, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return i
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
