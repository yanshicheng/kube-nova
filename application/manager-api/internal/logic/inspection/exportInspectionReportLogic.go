// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExportInspectionReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	w      http.ResponseWriter
}

type inspectionExportStyles struct {
	title     int
	section   int
	label     int
	value     int
	kpiTitle  int
	kpiValue  int
	header    int
	row       int
	success   int
	warning   int
	danger    int
	info      int
	wrap      int
	scoreGood int
	scoreWarn int
	scoreBad  int
}

type inspectionCategoryExportStat struct {
	Category      string
	TotalCount    int64
	SuccessCount  int64
	WarningCount  int64
	CriticalCount int64
	FailedCount   int64
}

type inspectionExportDetailRow struct {
	Category        string
	ItemName        string
	ItemCode        string
	TargetType      string
	TargetName      string
	Severity        string
	Status          string
	Score           float64
	Instance        string
	NodeName        string
	HostName        string
	ClusterName     string
	ClusterUuid     string
	ProjectName     string
	ProjectUuid     string
	WorkspaceName   string
	Namespace       string
	ApplicationName string
	ApplicationEn   string
	ResourceType    string
	ResourceName    string
	Version         string
	PodName         string
	Phase           string
	Reason          string
	Expected        string
	Actual          string
	Value           string
	Unit            string
	Message         string
	Suggestion      string
	DetailText      string
	DetailJson      string
}

type inspectionExportSummaryRow struct {
	Key             string
	TargetType      string
	TargetName      string
	ItemName        string
	ItemCode        string
	Category        string
	Status          string
	TotalCount      int64
	SuccessCount    int64
	WarningCount    int64
	CriticalCount   int64
	FailedCount     int64
	AbnormalObjects map[string]struct{}
}

// 导出巡检报告Excel
func NewExportInspectionReportLogic(ctx context.Context, svcCtx *svc.ServiceContext, w http.ResponseWriter) *ExportInspectionReportLogic {
	return &ExportInspectionReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		w:      w,
	}
}

func (l *ExportInspectionReportLogic) ExportInspectionReport(req *types.InspectionReportRequest) error {
	report, err := l.svcCtx.ManagerRpc.InspectionReportGet(l.ctx, &pb.InspectionReportReq{RecordId: req.RecordId})
	if err != nil {
		return err
	}
	if report == nil || report.Record == nil {
		return fmt.Errorf("巡检报告不存在")
	}
	if report.Record.Status == "running" {
		return fmt.Errorf("巡检报告仍在执行中，请完成后导出")
	}
	f := excelize.NewFile()
	defer f.Close()

	overviewSheet := "报告概览"
	detailSheet := "执行明细"
	targetSheet := "目标汇总"
	ruleSheet := "规则汇总"
	f.SetSheetName("Sheet1", overviewSheet)
	if _, err := f.NewSheet(detailSheet); err != nil {
		return fmt.Errorf("创建详细数据工作表失败: %v", err)
	}
	if _, err := f.NewSheet(targetSheet); err != nil {
		return fmt.Errorf("创建目标汇总工作表失败: %v", err)
	}
	if _, err := f.NewSheet(ruleSheet); err != nil {
		return fmt.Errorf("创建规则汇总工作表失败: %v", err)
	}
	if index, err := f.GetSheetIndex(overviewSheet); err == nil {
		f.SetActiveSheet(index)
	}

	styles := newInspectionExportStyles(f)
	trendRecords := l.loadTrendRecords(report.Record)
	detailRows := buildInspectionDetailRows(report)
	categoryStats := buildInspectionCategoryStats(report.Results)

	writeInspectionOverviewSheet(f, overviewSheet, report, trendRecords, categoryStats, styles)
	writeInspectionDetailSheet(f, detailSheet, detailRows, styles)
	writeInspectionSummarySheet(f, targetSheet, buildInspectionTargetSummaryRows(detailRows), styles, "target")
	writeInspectionSummarySheet(f, ruleSheet, buildInspectionRuleSummaryRows(detailRows), styles, "rule")

	fileName := fmt.Sprintf("巡检报告_%s.xlsx", report.Record.RecordNo)
	l.w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	l.w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileName)))
	if err := f.Write(l.w); err != nil {
		return fmt.Errorf("写出Excel失败: %v", err)
	}

	return nil
}

func (l *ExportInspectionReportLogic) loadTrendRecords(record *pb.InspectionRecord) []*pb.InspectionRecord {
	if record == nil {
		return nil
	}
	resp, err := l.svcCtx.ManagerRpc.InspectionRecordSearch(l.ctx, &pb.InspectionRecordSearchReq{
		ClusterUuid: record.ClusterUuid,
		TaskId:      record.TaskId,
		Page:        1,
		PageSize:    30,
		OrderField:  "id",
		IsAsc:       false,
	})
	if err != nil || resp == nil || len(resp.Data) == 0 {
		return []*pb.InspectionRecord{record}
	}
	items := append([]*pb.InspectionRecord(nil), resp.Data...)
	sort.SliceStable(items, func(i, j int) bool {
		return recordTime(items[i]) < recordTime(items[j])
	})
	return items
}

func newInspectionExportStyles(f *excelize.File) inspectionExportStyles {
	border := []excelize.Border{
		{Type: "left", Color: "#DADFE6", Style: 1},
		{Type: "top", Color: "#DADFE6", Style: 1},
		{Type: "right", Color: "#DADFE6", Style: 1},
		{Type: "bottom", Color: "#DADFE6", Style: 1},
	}
	title, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 18, Color: "#1F2937", Family: "Microsoft YaHei"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	section, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "#1F2937", Family: "Microsoft YaHei"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#EEF2F7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	label, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#4B5563", Family: "Microsoft YaHei"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F8FAFC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	value, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#111827", Family: "Microsoft YaHei"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    border,
	})
	kpiTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#64748B", Family: "Microsoft YaHei"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F8FAFC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	kpiValue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "#111827", Family: "Microsoft YaHei"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	header, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Family: "Microsoft YaHei"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#334155"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	row, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#111827", Family: "Microsoft YaHei"},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    border,
	})
	wrap, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#111827", Family: "Microsoft YaHei"},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Border:    border,
	})
	success, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#047857", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#ECFDF5"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	warning, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#B45309", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFFBEB"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	danger, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#B91C1C", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#FEF2F2"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	info, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#475569", Family: "Microsoft YaHei"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#F1F5F9"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	scoreGood, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#047857", Family: "Microsoft YaHei"}, NumFmt: 2, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	scoreWarn, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#B45309", Family: "Microsoft YaHei"}, NumFmt: 2, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	scoreBad, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#B91C1C", Family: "Microsoft YaHei"}, NumFmt: 2, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}, Border: border})
	return inspectionExportStyles{
		title:     title,
		section:   section,
		label:     label,
		value:     value,
		kpiTitle:  kpiTitle,
		kpiValue:  kpiValue,
		header:    header,
		row:       row,
		wrap:      wrap,
		success:   success,
		warning:   warning,
		danger:    danger,
		info:      info,
		scoreGood: scoreGood,
		scoreWarn: scoreWarn,
		scoreBad:  scoreBad,
	}
}

func writeInspectionOverviewSheet(f *excelize.File, sheet string, report *pb.InspectionReportResp, trendRecords []*pb.InspectionRecord, categoryStats []inspectionCategoryExportStat, styles inspectionExportStyles) {
	record := report.Record
	f.MergeCell(sheet, "A1", "J1")
	f.SetCellValue(sheet, "A1", "自动化集群巡检报告")
	f.SetCellStyle(sheet, "A1", "J1", styles.title)
	f.SetRowHeight(sheet, 1, 30)

	writeSection(f, sheet, "A3", "D3", "基础信息", styles.section)
	baseRows := [][]any{
		{"报告编号", record.RecordNo, "集群名称", record.ClusterName},
		{"集群UUID", record.ClusterUuid, "执行状态", statusText(record.Status)},
		{"健康等级", healthText(record.HealthLevel), "巡检得分", record.Score},
		{"触发类型", triggerText(record.TriggerType), "巡检范围", scopeText(record.ScopeType)},
		{"开始时间", formatUnix(record.StartedAt), "结束时间", formatUnix(record.FinishedAt)},
		{"执行耗时", formatDuration(record.DurationMs), "异常信息", record.ErrorMessage},
	}
	for i, row := range baseRows {
		rowNo := i + 4
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNo)
			f.SetCellValue(sheet, cell, value)
			if colIdx%2 == 0 {
				f.SetCellStyle(sheet, cell, cell, styles.label)
			} else {
				f.SetCellStyle(sheet, cell, cell, styles.value)
			}
		}
	}

	writeSection(f, sheet, "A12", "J12", "整体统计", styles.section)
	abnormalCount := record.WarningCount + record.CriticalCount + record.FailedCount
	kpiTitles := []string{"检查总数", "成功数", "异常数", "警告数", "严重数", "失败数", "得分", "健康等级", "通过率"}
	kpiValues := []any{record.TotalCount, record.SuccessCount, abnormalCount, record.WarningCount, record.CriticalCount, record.FailedCount, record.Score, healthText(record.HealthLevel), passRateText(record.SuccessCount, record.TotalCount)}
	for i, title := range kpiTitles {
		cell, _ := excelize.CoordinatesToCellName(i+1, 13)
		f.SetCellValue(sheet, cell, title)
		f.SetCellStyle(sheet, cell, cell, styles.kpiTitle)
		valueCell, _ := excelize.CoordinatesToCellName(i+1, 14)
		f.SetCellValue(sheet, valueCell, kpiValues[i])
		f.SetCellStyle(sheet, valueCell, valueCell, styles.kpiValue)
	}

	writeSection(f, sheet, "A17", "F17", "分类统计", styles.section)
	categoryHeaders := []string{"分类", "检查总数", "成功数", "异常数", "警告数", "严重数", "失败数", "通过率"}
	writeHeaderRow(f, sheet, 18, categoryHeaders, styles.header)
	for i, item := range categoryStats {
		rowNo := i + 19
		abnormal := item.WarningCount + item.CriticalCount + item.FailedCount
		values := []any{item.Category, item.TotalCount, item.SuccessCount, abnormal, item.WarningCount, item.CriticalCount, item.FailedCount, passRateText(item.SuccessCount, item.TotalCount)}
		writeStyledRow(f, sheet, rowNo, values, styles.row)
	}

	writeStatusSource(f, sheet, report, styles)
	writeTrendSource(f, sheet, trendRecords, styles)
	addOverviewCharts(f, sheet, len(trendRecords))

	f.SetColWidth(sheet, "A", "A", 16)
	f.SetColWidth(sheet, "B", "B", 24)
	f.SetColWidth(sheet, "C", "C", 16)
	f.SetColWidth(sheet, "D", "D", 34)
	f.SetColWidth(sheet, "E", "J", 14)
	f.SetColWidth(sheet, "L", "Q", 16)
}

func writeInspectionDetailSheet(f *excelize.File, sheet string, rows []inspectionExportDetailRow, styles inspectionExportStyles) {
	headers := []string{
		"定位路径", "分类", "巡检项", "目标名称", "node", "集群名称", "项目", "资源名称", "pod",
		"期望值", "指标值", "级别", "状态", "得分", "说明", "建议", "明细",
	}
	widths := []float64{
		42, 16, 24, 28, 24, 24, 18, 24, 22,
		22, 16, 12, 12, 10, 34, 34, 42,
	}
	rowValues := make([][]any, 0, len(rows))
	for _, item := range rows {
		rowValues = append(rowValues, inspectionDetailValues(item))
	}
	visibleColumns := visibleInspectionDetailColumns(headers, rowValues)
	visibleHeaders := make([]string, 0, len(visibleColumns))
	for _, sourceCol := range visibleColumns {
		visibleHeaders = append(visibleHeaders, headers[sourceCol])
	}
	writeHeaderRow(f, sheet, 1, visibleHeaders, styles.header)
	for rowIdx, item := range rows {
		rowNo := rowIdx + 2
		values := rowValues[rowIdx]
		for colIdx, sourceCol := range visibleColumns {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNo)
			value := values[sourceCol]
			f.SetCellValue(sheet, cell, value)
			switch sourceCol {
			case 11, 12:
				f.SetCellStyle(sheet, cell, cell, statusCellStyle(item.Status, item.Severity, styles))
			case 13:
				f.SetCellStyle(sheet, cell, cell, scoreCellStyle(item.Score, styles))
			case 0, 14, 15, 16:
				f.SetCellStyle(sheet, cell, cell, styles.wrap)
			default:
				f.SetCellStyle(sheet, cell, cell, styles.row)
			}
		}
	}
	lastCol, _ := excelize.CoordinatesToCellName(len(visibleColumns), maxInt(2, len(rows)+1))
	f.AutoFilter(sheet, fmt.Sprintf("A1:%s", lastCol), nil)
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
	for colIdx, sourceCol := range visibleColumns {
		colName, _ := excelize.ColumnNumberToName(colIdx + 1)
		f.SetColWidth(sheet, colName, colName, widths[sourceCol])
	}
}

func writeInspectionSummarySheet(f *excelize.File, sheet string, rows []inspectionExportSummaryRow, styles inspectionExportStyles, mode string) {
	headers := []string{"目标类型", "目标名称", "巡检项", "规则编码", "分类", "状态", "检查总数", "成功数", "异常数", "警告数", "严重数", "失败数", "异常明细"}
	if mode == "rule" {
		headers = []string{"巡检项", "规则编码", "分类", "状态", "目标总数", "成功数", "异常数", "警告数", "严重数", "失败数", "异常目标"}
	}
	writeHeaderRow(f, sheet, 1, headers, styles.header)
	for i, row := range rows {
		rowNo := i + 2
		abnormal := row.WarningCount + row.CriticalCount + row.FailedCount
		objects := joinSummaryObjects(row.AbnormalObjects)
		values := []any{
			targetTypeText(row.TargetType), row.TargetName, row.ItemName, row.ItemCode, row.Category, statusText(row.Status),
			row.TotalCount, row.SuccessCount, abnormal, row.WarningCount, row.CriticalCount, row.FailedCount, objects,
		}
		if mode == "rule" {
			values = []any{
				row.ItemName, row.ItemCode, row.Category, statusText(row.Status), row.TotalCount, row.SuccessCount,
				abnormal, row.WarningCount, row.CriticalCount, row.FailedCount, objects,
			}
		}
		for colIdx, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNo)
			f.SetCellValue(sheet, cell, value)
			if (mode == "target" && colIdx == 5) || (mode == "rule" && colIdx == 3) {
				f.SetCellStyle(sheet, cell, cell, statusCellStyle(row.Status, "", styles))
			} else if colIdx == len(values)-1 {
				f.SetCellStyle(sheet, cell, cell, styles.wrap)
			} else {
				f.SetCellStyle(sheet, cell, cell, styles.row)
			}
		}
	}
	lastCol, _ := excelize.CoordinatesToCellName(len(headers), maxInt(2, len(rows)+1))
	f.AutoFilter(sheet, fmt.Sprintf("A1:%s", lastCol), nil)
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		width := 16.0
		if i == 1 || i == len(headers)-1 {
			width = 34
		}
		f.SetColWidth(sheet, col, col, width)
	}
}

func inspectionDetailValues(item inspectionExportDetailRow) []any {
	return []any{
		inspectionGroupPath(item), item.Category, item.ItemName,
		item.TargetName,
		firstNonEmpty(item.NodeName, item.HostName),
		item.ClusterName, item.ProjectName, item.ResourceName, item.PodName,
		item.Expected, inspectionMetricValueText(item), severityText(item.Severity), statusText(item.Status), item.Score,
		item.Message, item.Suggestion, item.DetailText,
	}
}

func visibleInspectionDetailColumns(headers []string, rows [][]any) []int {
	alwaysKeep := map[int]struct{}{
		0: {}, 1: {}, 2: {}, 3: {},
		9: {}, 10: {}, 11: {}, 12: {}, 13: {}, 14: {},
	}
	visible := make([]int, 0, len(headers))
	for colIdx := range headers {
		if _, ok := alwaysKeep[colIdx]; ok {
			visible = append(visible, colIdx)
			continue
		}
		for _, row := range rows {
			if colIdx < len(row) && strings.TrimSpace(fmt.Sprint(row[colIdx])) != "" {
				visible = append(visible, colIdx)
				break
			}
		}
	}
	return visible
}

func buildInspectionCategoryStats(results []*pb.InspectionResult) []inspectionCategoryExportStat {
	stats := make(map[string]*inspectionCategoryExportStat)
	for _, result := range results {
		category := strings.TrimSpace(result.Category)
		if category == "" {
			category = "默认分类"
		}
		stat := stats[category]
		if stat == nil {
			stat = &inspectionCategoryExportStat{Category: category}
			stats[category] = stat
		}
		stat.TotalCount++
		switch result.Status {
		case "critical":
			stat.CriticalCount++
		case "warning":
			stat.WarningCount++
		case "failed":
			stat.FailedCount++
		default:
			stat.SuccessCount++
		}
	}
	items := make([]inspectionCategoryExportStat, 0, len(stats))
	for _, item := range stats {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		leftRisk := items[i].WarningCount + items[i].CriticalCount + items[i].FailedCount
		rightRisk := items[j].WarningCount + items[j].CriticalCount + items[j].FailedCount
		if leftRisk != rightRisk {
			return leftRisk > rightRisk
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func buildInspectionDetailRows(report *pb.InspectionReportResp) []inspectionExportDetailRow {
	rows := make([]inspectionExportDetailRow, 0, len(report.Results))
	for _, result := range report.Results {
		details := parseInspectionDetailJson(result.DetailJson)
		if len(details) == 0 {
			rows = append(rows, newInspectionDetailRow(report.Record, result, nil))
			continue
		}
		for _, detail := range details {
			rows = append(rows, newInspectionDetailRow(report.Record, result, detail))
		}
	}
	normalizeInspectionHostRows(rows)
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		keys := []int{
			strings.Compare(left.ClusterName, right.ClusterName),
			strings.Compare(left.ProjectName, right.ProjectName),
			strings.Compare(left.WorkspaceName, right.WorkspaceName),
			strings.Compare(left.ApplicationName, right.ApplicationName),
			strings.Compare(left.Namespace, right.Namespace),
			strings.Compare(left.ResourceType, right.ResourceType),
			strings.Compare(left.ResourceName, right.ResourceName),
			statusRank(left.Status) - statusRank(right.Status),
			strings.Compare(left.Category, right.Category),
			strings.Compare(left.ItemName, right.ItemName),
		}
		for _, value := range keys {
			if value != 0 {
				return value < 0
			}
		}
		return false
	})
	return rows
}

func buildInspectionTargetSummaryRows(rows []inspectionExportDetailRow) []inspectionExportSummaryRow {
	stats := make(map[string]*inspectionExportSummaryRow)
	for _, row := range rows {
		key := row.TargetType + ":" + firstNonEmpty(row.TargetName, row.Instance, row.NodeName, row.PodName, row.ResourceName)
		item := stats[key]
		if item == nil {
			item = &inspectionExportSummaryRow{
				Key:             key,
				TargetType:      row.TargetType,
				TargetName:      firstNonEmpty(row.TargetName, row.Instance, row.NodeName, row.PodName, row.ResourceName),
				Status:          "success",
				AbnormalObjects: make(map[string]struct{}),
			}
			stats[key] = item
		}
		applySummaryStatus(item, row.Status)
		if row.Status != "success" {
			item.AbnormalObjects[firstNonEmpty(row.ItemName, row.ItemCode)] = struct{}{}
		}
	}
	return sortInspectionSummaryRows(stats)
}

func buildInspectionRuleSummaryRows(rows []inspectionExportDetailRow) []inspectionExportSummaryRow {
	stats := make(map[string]*inspectionExportSummaryRow)
	for _, row := range rows {
		key := row.ItemCode
		if key == "" {
			key = row.ItemName
		}
		item := stats[key]
		if item == nil {
			item = &inspectionExportSummaryRow{
				Key:             key,
				ItemName:        row.ItemName,
				ItemCode:        row.ItemCode,
				Category:        row.Category,
				Status:          "success",
				AbnormalObjects: make(map[string]struct{}),
			}
			stats[key] = item
		}
		applySummaryStatus(item, row.Status)
		if row.Status != "success" {
			item.AbnormalObjects[firstNonEmpty(row.TargetName, row.Instance, row.NodeName, row.PodName, row.ResourceName)] = struct{}{}
		}
	}
	return sortInspectionSummaryRows(stats)
}

func applySummaryStatus(item *inspectionExportSummaryRow, status string) {
	item.TotalCount++
	switch status {
	case "critical":
		item.CriticalCount++
	case "warning":
		item.WarningCount++
	case "failed":
		item.FailedCount++
	default:
		item.SuccessCount++
	}
	if statusRank(status) < statusRank(item.Status) {
		item.Status = status
	}
}

func sortInspectionSummaryRows(stats map[string]*inspectionExportSummaryRow) []inspectionExportSummaryRow {
	rows := make([]inspectionExportSummaryRow, 0, len(stats))
	for _, item := range stats {
		rows = append(rows, *item)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if statusRank(rows[i].Status) != statusRank(rows[j].Status) {
			return statusRank(rows[i].Status) < statusRank(rows[j].Status)
		}
		return rows[i].Key < rows[j].Key
	})
	return rows
}

func joinSummaryObjects(values map[string]struct{}) string {
	if len(values) == 0 {
		return ""
	}
	items := make([]string, 0, len(values))
	for item := range values {
		if strings.TrimSpace(item) != "" {
			items = append(items, item)
		}
	}
	sort.Strings(items)
	return strings.Join(items, "、")
}

func normalizeInspectionHostRows(rows []inspectionExportDetailRow) {
	hostNames := make(map[string]string)
	for _, row := range rows {
		if !isHostTarget(row.TargetType) {
			continue
		}
		for _, value := range []string{row.TargetName, row.NodeName, row.HostName, row.Instance, row.ResourceName} {
			if !looksLikeHostName(value) {
				continue
			}
			if key := hostIdentityKey(value); key != "" {
				if _, exists := hostNames[key]; !exists {
					hostNames[key] = strings.TrimSpace(value)
				}
			}
		}
	}
	for i := range rows {
		if !isHostTarget(rows[i].TargetType) {
			continue
		}
		canonical := firstNonEmpty(hostNameCandidate(rows[i].TargetName), hostNameCandidate(rows[i].NodeName), hostNameCandidate(rows[i].HostName), hostNameCandidate(rows[i].Instance))
		if canonical == "" {
			for _, value := range []string{rows[i].TargetName, rows[i].NodeName, rows[i].HostName, rows[i].Instance, rows[i].ResourceName} {
				if key := hostIdentityKey(value); key != "" {
					if name := hostNames[key]; name != "" {
						canonical = name
						break
					}
				}
			}
		}
		if canonical == "" {
			continue
		}
		rows[i].TargetName = canonical
		rows[i].Instance = canonical
		rows[i].NodeName = canonical
		rows[i].HostName = ""
		if rows[i].ResourceName != "" && isHostIdentity(rows[i].ResourceName) {
			rows[i].ResourceName = ""
		}
	}
}

func isHostTarget(targetType string) bool {
	return strings.EqualFold(targetType, "host") || strings.EqualFold(targetType, "node")
}

func hostNameCandidate(value string) string {
	if looksLikeHostName(value) {
		return strings.TrimSpace(value)
	}
	return ""
}

func looksLikeHostName(value string) bool {
	value = stripHostPort(strings.TrimSpace(value))
	if value == "" || isIPv4(value) {
		return false
	}
	for _, ch := range value {
		if ch > 127 {
			return false
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return true
		}
	}
	return false
}

func isHostIdentity(value string) bool {
	value = stripHostPort(strings.TrimSpace(value))
	return value != "" && (isIPv4(value) || looksLikeHostName(value))
}

func hostIdentityKey(value string) string {
	value = stripHostPort(strings.ToLower(strings.TrimSpace(value)))
	if value == "" {
		return ""
	}
	if isIPv4(value) {
		parts := strings.Split(value, ".")
		return parts[len(parts)-1]
	}
	current := ""
	last := ""
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			current += string(ch)
			continue
		}
		if current != "" {
			last = current
			current = ""
		}
	}
	if current != "" {
		last = current
	}
	return strings.TrimLeft(last, "0")
}

func stripHostPort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "/") {
		return value
	}
	if idx := strings.LastIndex(value, ":"); idx > 0 && idx < len(value)-1 {
		if _, err := strconv.Atoi(value[idx+1:]); err == nil {
			return value[:idx]
		}
	}
	return value
}

func isIPv4(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func newInspectionDetailRow(record *pb.InspectionRecord, result *pb.InspectionResult, detail map[string]string) inspectionExportDetailRow {
	row := inspectionExportDetailRow{
		Category:    result.Category,
		ItemName:    result.ItemName,
		ItemCode:    result.ItemCode,
		TargetType:  result.TargetType,
		TargetName:  result.TargetName,
		Severity:    result.Severity,
		Status:      result.Status,
		Score:       result.Score,
		ClusterName: record.ClusterName,
		ClusterUuid: record.ClusterUuid,
		Expected:    result.Expected,
		Actual:      result.Actual,
		Value:       result.Value,
		Unit:        result.Unit,
		Message:     result.Message,
		Suggestion:  result.Suggestion,
		DetailJson:  result.DetailJson,
	}
	if len(detail) == 0 {
		normalizeInspectionBusinessScope(&row)
		return row
	}
	row.ClusterName = firstNonEmpty(detail["clusterName"], row.ClusterName)
	row.ClusterUuid = firstNonEmpty(detail["clusterUuid"], row.ClusterUuid)
	row.ProjectName = detail["projectName"]
	row.ProjectUuid = detail["projectUuid"]
	row.WorkspaceName = detail["workspaceName"]
	row.Namespace = detail["namespace"]
	row.ApplicationName = detail["applicationName"]
	row.ApplicationEn = detail["applicationEn"]
	row.ResourceType = detail["resourceType"]
	row.ResourceName = detail["resourceName"]
	row.Version = detail["version"]
	row.Value = firstNonEmpty(detail["value"], row.Value)
	row.Unit = firstNonEmpty(detail["unit"], row.Unit)
	row.Instance = firstNonEmpty(detail["instance"], detail["address"])
	row.NodeName = firstNonEmpty(detail["nodeName"], detail["node"], detail["nodename"], detail["kubernetes_node"])
	row.HostName = firstNonEmpty(detail["hostName"], detail["host"], detail["hostname"])
	if isHostTarget(result.TargetType) {
		row.NodeName = firstNonEmpty(row.NodeName, detail["name"])
		if row.NodeName == "" && looksLikeHostName(row.Instance) {
			row.NodeName = row.Instance
		}
	} else {
		row.PodName = firstNonEmpty(detail["podName"], detail["pod"])
		if row.PodName == "" && strings.EqualFold(result.TargetType, "pod") {
			row.PodName = detail["name"]
		}
	}
	if strings.TrimSpace(row.ResourceName) == "" {
		row.ResourceName = firstNonEmpty(detail["service"], detail["deployment"], detail["statefulset"], detail["daemonset"], detail["job"])
	}
	if strings.TrimSpace(row.TargetName) == "" {
		if isHostTarget(result.TargetType) {
			row.TargetName = firstNonEmpty(row.NodeName, row.HostName, row.Instance, row.PodName, row.ResourceName)
		} else {
			row.TargetName = firstNonEmpty(row.NodeName, row.HostName, row.PodName, row.Instance, row.ResourceName)
		}
	}
	row.Phase = detail["phase"]
	row.Reason = detail["reason"]
	row.DetailText = inspectionDetailText(detail)
	normalizeInspectionBusinessScope(&row)
	return row
}

func inspectionMetricValueText(row inspectionExportDetailRow) string {
	valueText := strings.TrimSpace(row.Value)
	if valueText == "" {
		return ""
	}
	unit := strings.TrimSpace(row.Unit)
	if unit == "" {
		return valueText
	}
	return valueText + " " + unit
}

func inspectionActualText(row inspectionExportDetailRow) string {
	valueText := strings.TrimSpace(row.Value)
	if row.Unit != "" && valueText != "" {
		valueText += " " + row.Unit
	}
	parts := make([]string, 0, 4)
	if valueText != "" {
		parts = append(parts, "指标值 "+valueText)
	}
	if row.Expected != "" {
		parts = append(parts, "期望 "+row.Expected)
	}
	if row.Actual != "" && (row.Value == "" || !strings.Contains(row.Actual, row.Value)) {
		parts = append(parts, row.Actual)
	}
	if row.DetailText != "" {
		parts = append(parts, row.DetailText)
	}
	return firstNonEmpty(strings.Join(parts, "\n"), row.Message, "-")
}

func normalizeInspectionBusinessScope(row *inspectionExportDetailRow) {
	if row == nil {
		return
	}
	row.ProjectName = strings.TrimSpace(row.ProjectName)
	row.WorkspaceName = strings.TrimSpace(row.WorkspaceName)
	row.Namespace = strings.TrimSpace(row.Namespace)
}

func inspectionGroupPath(row inspectionExportDetailRow) string {
	parts := []string{
		firstNonEmpty(row.ClusterName, row.ClusterUuid, "集群级别"),
		firstNonEmpty(row.ProjectName, "集群级别"),
		firstNonEmpty(row.WorkspaceName, row.Namespace, "集群级别"),
		firstNonEmpty(row.ApplicationName, row.ApplicationEn),
		firstNonEmpty(row.ResourceName, row.PodName, row.NodeName, row.HostName, row.Instance),
	}
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return strings.Join(out, " / ")
}

func parseInspectionDetailJson(raw string) []map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(raw), &list); err == nil {
		return normalizeDetailList(list)
	}
	var single map[string]any
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		return normalizeDetailList([]map[string]any{single})
	}
	return nil
}

func normalizeDetailList(source []map[string]any) []map[string]string {
	result := make([]map[string]string, 0, len(source))
	for _, item := range source {
		row := make(map[string]string, len(item))
		for key, value := range item {
			switch v := value.(type) {
			case string:
				row[key] = v
			case float64:
				if v == float64(int64(v)) {
					row[key] = strconv.FormatInt(int64(v), 10)
				} else {
					row[key] = strconv.FormatFloat(v, 'f', 2, 64)
				}
			case bool:
				row[key] = strconv.FormatBool(v)
			default:
				b, _ := json.Marshal(v)
				row[key] = string(b)
			}
		}
		result = append(result, row)
	}
	return result
}

func inspectionDetailText(detail map[string]string) string {
	if len(detail) == 0 {
		return ""
	}
	hidden := map[string]struct{}{
		"clusterUuid": {}, "clusterName": {}, "projectId": {}, "projectUuid": {}, "projectName": {},
		"workspaceId": {}, "workspaceName": {}, "namespace": {}, "applicationId": {}, "applicationName": {},
		"applicationEn": {}, "resourceType": {}, "resourceName": {}, "versionId": {}, "version": {},
		"podName": {}, "pod": {}, "nodeName": {}, "node": {}, "nodename": {}, "kubernetes_node": {},
		"hostName": {}, "host": {}, "hostname": {}, "instance": {}, "endpoint": {}, "name": {}, "value": {}, "unit": {},
		"phase": {}, "reason": {}, "targetType": {}, "targetName": {}, "targetKind": {}, "detailPromql": {}, "detailOperator": {}, "detailThreshold": {},
		"detailExpected": {}, "conditionSource": {}, "hasTargetLabels": {}, "prometheusInstance": {}, "seriesCount": {}, "noData": {},
	}
	keys := make([]string, 0, len(detail))
	for key, value := range detail {
		if _, ok := hidden[key]; ok || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", inspectionDetailLabel(key), detail[key]))
	}
	return strings.Join(parts, "\n")
}

func inspectionDetailLabel(key string) string {
	switch key {
	case "status":
		return "状态"
	case "type":
		return "类型"
	case "condition":
		return "条件"
	case "message":
		return "信息"
	case "metricName":
		return "指标"
	case "job":
		return "任务"
	case "service":
		return "服务"
	case "endpoint":
		return "端点"
	case "device":
		return "设备"
	case "mountpoint":
		return "挂载点"
	case "container":
		return "容器"
	default:
		return key
	}
}

func writeStatusSource(f *excelize.File, sheet string, report *pb.InspectionReportResp, styles inspectionExportStyles) {
	f.SetCellValue(sheet, "L3", "状态")
	f.SetCellValue(sheet, "M3", "数量")
	f.SetCellValue(sheet, "N3", "占比(%)")
	f.SetCellStyle(sheet, "L3", "N3", styles.header)
	counts := []int64{
		report.Record.SuccessCount,
		report.Record.WarningCount,
		report.Record.CriticalCount,
		report.Record.FailedCount,
	}
	percents := statusDistributionPercentages(counts)
	rows := [][]any{
		{"正常", counts[0], percents[0]},
		{"警告", counts[1], percents[1]},
		{"严重", counts[2], percents[2]},
		{"失败", counts[3], percents[3]},
	}
	for i, row := range rows {
		writeStyledRow(f, sheet, i+4, row, styles.row, 12)
	}
}

func writeTrendSource(f *excelize.File, sheet string, records []*pb.InspectionRecord, styles inspectionExportStyles) {
	f.SetCellValue(sheet, "O3", "时间")
	f.SetCellValue(sheet, "P3", "得分")
	f.SetCellValue(sheet, "Q3", "风险项")
	f.SetCellStyle(sheet, "O3", "Q3", styles.header)
	for i, record := range records {
		rowNo := i + 4
		f.SetCellValue(sheet, fmt.Sprintf("O%d", rowNo), formatUnix(recordTime(record)))
		f.SetCellValue(sheet, fmt.Sprintf("P%d", rowNo), record.Score)
		f.SetCellValue(sheet, fmt.Sprintf("Q%d", rowNo), record.WarningCount+record.CriticalCount+record.FailedCount)
		f.SetCellStyle(sheet, fmt.Sprintf("O%d", rowNo), fmt.Sprintf("Q%d", rowNo), styles.row)
	}
}

func addOverviewCharts(f *excelize.File, sheet string, trendCount int) {
	_ = f.AddChart(sheet, "F3", &excelize.Chart{
		Type: excelize.Bar,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("'%s'!$N$3", sheet),
			Categories: fmt.Sprintf("'%s'!$L$4:$L$7", sheet),
			Values:     fmt.Sprintf("'%s'!$N$4:$N$7", sheet),
		}},
		Dimension: excelize.ChartDimension{Width: 420, Height: 230},
		Legend:    excelize.ChartLegend{Position: "none"},
		Title:     []excelize.RichTextRun{{Text: "结果状态占比(%)"}},
	})
	if trendCount <= 0 {
		return
	}
	endRow := trendCount + 3
	_ = f.AddChart(sheet, "F17", &excelize.Chart{
		Type: excelize.Line,
		Series: []excelize.ChartSeries{
			{
				Name:       fmt.Sprintf("'%s'!$P$3", sheet),
				Categories: fmt.Sprintf("'%s'!$O$4:$O$%d", sheet, endRow),
				Values:     fmt.Sprintf("'%s'!$P$4:$P$%d", sheet, endRow),
			},
			{
				Name:       fmt.Sprintf("'%s'!$Q$3", sheet),
				Categories: fmt.Sprintf("'%s'!$O$4:$O$%d", sheet, endRow),
				Values:     fmt.Sprintf("'%s'!$Q$4:$Q$%d", sheet, endRow),
			},
		},
		Dimension: excelize.ChartDimension{Width: 520, Height: 260},
		Legend:    excelize.ChartLegend{Position: "bottom"},
		Title:     []excelize.RichTextRun{{Text: "近 30 次巡检趋势"}},
	})
}

func statusDistributionPercentages(counts []int64) []float64 {
	percents := make([]float64, len(counts))
	var total int64
	lastNonZero := -1
	for i, count := range counts {
		total += count
		if count > 0 {
			lastNonZero = i
		}
	}
	if total <= 0 {
		return percents
	}
	var used float64
	for i, count := range counts {
		if count <= 0 {
			continue
		}
		if i == lastNonZero {
			percents[i] = math.Round((100-used)*10) / 10
			continue
		}
		percent := math.Round(float64(count)/float64(total)*1000) / 10
		percents[i] = percent
		used += percent
	}
	return percents
}

func writeSection(f *excelize.File, sheet, startCell, endCell, title string, style int) {
	f.MergeCell(sheet, startCell, endCell)
	f.SetCellValue(sheet, startCell, title)
	f.SetCellStyle(sheet, startCell, endCell, style)
}

func writeHeaderRow(f *excelize.File, sheet string, rowNo int, headers []string, style int) {
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, rowNo)
		f.SetCellValue(sheet, cell, header)
		f.SetCellStyle(sheet, cell, cell, style)
	}
}

func writeStyledRow(f *excelize.File, sheet string, rowNo int, values []any, style int, offset ...int) {
	startCol := 1
	if len(offset) > 0 {
		startCol = offset[0]
	}
	for i, value := range values {
		cell, _ := excelize.CoordinatesToCellName(startCol+i, rowNo)
		f.SetCellValue(sheet, cell, value)
		f.SetCellStyle(sheet, cell, cell, style)
	}
}

func statusCellStyle(status, severity string, styles inspectionExportStyles) int {
	switch {
	case status == "success":
		return styles.success
	case status == "critical" || severity == "critical":
		return styles.danger
	case status == "warning" || severity == "warning":
		return styles.warning
	case status == "failed":
		return styles.danger
	default:
		return styles.info
	}
}

func scoreCellStyle(score float64, styles inspectionExportStyles) int {
	switch {
	case score >= 90:
		return styles.scoreGood
	case score >= 70:
		return styles.scoreWarn
	default:
		return styles.scoreBad
	}
}

func recordTime(record *pb.InspectionRecord) int64 {
	if record == nil {
		return 0
	}
	if record.StartedAt > 0 {
		return record.StartedAt
	}
	return record.CreatedAt
}

func statusRank(status string) int {
	switch status {
	case "critical", "failed":
		return 0
	case "warning", "partial":
		return 1
	case "running":
		return 2
	case "success":
		return 3
	default:
		return 4
	}
}

func scopeText(value string) string {
	switch value {
	case "global":
		return "全局巡检"
	case "cluster":
		return "单集群"
	default:
		return firstNonEmpty(value, "-")
	}
}

func triggerText(value string) string {
	switch value {
	case "schedule":
		return "定时"
	case "manual":
		return "手动"
	default:
		return firstNonEmpty(value, "-")
	}
}

func targetTypeText(value string) string {
	switch value {
	case "cluster":
		return "集群"
	case "host", "node":
		return "主机"
	case "pod":
		return "Pod"
	case "workload":
		return "工作负载"
	case "database":
		return "数据库"
	case "middleware", "etcd", "kafka", "rabbitmq", "zookeeper":
		return "中间件"
	case "mysql":
		return "数据库"
	case "redis":
		return "数据库"
	case "mongodb":
		return "数据库"
	case "postgresql":
		return "数据库"
	case "prometheus":
		return "Prometheus"
	default:
		return firstNonEmpty(value, "-")
	}
}

func severityText(value string) string {
	switch value {
	case "critical":
		return "严重"
	case "warning":
		return "警告"
	case "info":
		return "提示"
	default:
		return firstNonEmpty(value, "-")
	}
}

func statusText(value string) string {
	switch value {
	case "running":
		return "执行中"
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "partial":
		return "部分成功"
	case "warning":
		return "警告"
	case "critical":
		return "严重"
	default:
		return firstNonEmpty(value, "-")
	}
}

func healthText(value string) string {
	switch value {
	case "healthy":
		return "健康"
	case "warning":
		return "警告"
	case "critical":
		return "严重"
	case "unknown":
		return "未知"
	default:
		return firstNonEmpty(value, "-")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func passRateText(successCount, totalCount int64) string {
	if totalCount <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", float64(successCount)/float64(totalCount)*100)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func formatDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}
