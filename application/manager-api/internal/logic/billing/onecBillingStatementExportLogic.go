package billing

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xuri/excelize/v2"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingStatementExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementExportLogic {
	return &OnecBillingStatementExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingStatementExportLogic) OnecBillingStatementExport(req *types.OnecBillingStatementExportRequest, w http.ResponseWriter) error {
	// 调用RPC服务搜索账单（不分页，获取全部数据）
	result, err := l.svcCtx.ManagerRpc.OnecBillingStatementSearch(l.ctx, &pb.OnecBillingStatementSearchReq{
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		ClusterUuid:   req.ClusterUuid,
		ProjectId:     req.ProjectId,
		StatementType: req.StatementType,
		Page:          1,
		PageSize:      10000,
		OrderField:    req.OrderField,
		IsAsc:         req.IsAsc,
	})
	if err != nil {
		l.Errorf("导出账单失败，获取数据错误: %v", err)
		return fmt.Errorf("获取账单数据失败: %v", err)
	}

	// 创建 Excel 文件
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "账单明细"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		l.Errorf("创建工作表失败: %v", err)
		return fmt.Errorf("创建工作表失败: %v", err)
	}
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// 设置表头
	headers := []string{
		"账单编号", "账单类型", "集群名称", "项目名称",
		"计费开始时间", "计费结束时间", "计费时长(小时)",
		"CPU容量", "内存容量", "存储配额", "GPU容量", "Pod配额",
		"工作空间数", "应用数量",
		"CPU单价", "内存单价", "存储单价", "GPU单价", "Pod单价", "管理费单价",
		"CPU费用", "内存费用", "存储费用", "GPU费用", "Pod费用", "管理费",
		"资源费用合计", "总费用", "备注", "创建时间",
	}

	// ========== 创建样式 ==========

	// 表头样式 - 渐变蓝色背景
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   11,
			Color:  "#FFFFFF",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "gradient",
			Color:   []string{"#4F46E5", "#6366F1"},
			Shading: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#CBD5E1", Style: 1},
			{Type: "top", Color: "#CBD5E1", Style: 1},
			{Type: "bottom", Color: "#CBD5E1", Style: 2},
			{Type: "right", Color: "#CBD5E1", Style: 1},
		},
	})

	// 偶数行样式 - 白色背景
	evenRowStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:   10,
			Color:  "#1E293B",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FFFFFF"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 奇数行样式 - 浅灰背景
	oddRowStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:   10,
			Color:  "#1E293B",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#F8FAFC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 金额列样式（偶数行）- 绿色右对齐
	moneyStyleEven, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:   10,
			Color:  "#059669",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FFFFFF"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 金额列样式（奇数行）
	moneyStyleOdd, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:   10,
			Color:  "#059669",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#F8FAFC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 总费用列样式（偶数行）- 紫色加粗
	totalStyleEven, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   10,
			Color:  "#7C3AED",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FFFFFF"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 总费用列样式（奇数行）
	totalStyleOdd, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   10,
			Color:  "#7C3AED",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#F8FAFC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 写入表头
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 应用表头样式
	lastHeaderCol, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheetName, "A1", lastHeaderCol, headerStyle)
	f.SetRowHeight(sheetName, 1, 28)

	// 金额列索引（从0开始）: 14-19是单价列，20-27是费用列
	moneyColIndices := map[int]bool{
		14: true, 15: true, 16: true, 17: true, 18: true, 19: true, // 单价
		20: true, 21: true, 22: true, 23: true, 24: true, 25: true, 26: true, // 费用
	}
	totalAmountColIndex := 27 // 总费用列

	// 写入数据
	for rowIdx, item := range result.List {
		row := rowIdx + 2
		isOddRow := rowIdx%2 == 1

		statementTypeText := l.getStatementTypeText(item.StatementType)

		rowData := []interface{}{
			item.StatementNo,
			statementTypeText,
			item.ClusterName,
			item.ProjectName,
			l.formatTimestamp(item.BillingStartTime),
			l.formatTimestamp(item.BillingEndTime),
			fmt.Sprintf("%.2f", item.BillingHours),
			item.CpuCapacity,
			item.MemCapacity,
			item.StorageLimit,
			item.GpuCapacity,
			item.PodsLimit,
			item.WorkspaceCount,
			item.ApplicationCount,
			fmt.Sprintf("%.4f", item.PriceCpu),
			fmt.Sprintf("%.4f", item.PriceMemory),
			fmt.Sprintf("%.4f", item.PriceStorage),
			fmt.Sprintf("%.4f", item.PriceGpu),
			fmt.Sprintf("%.4f", item.PricePod),
			fmt.Sprintf("%.4f", item.PriceManagementFee),
			fmt.Sprintf("%.2f", item.CpuCost),
			fmt.Sprintf("%.2f", item.MemoryCost),
			fmt.Sprintf("%.2f", item.StorageCost),
			fmt.Sprintf("%.2f", item.GpuCost),
			fmt.Sprintf("%.2f", item.PodCost),
			fmt.Sprintf("%.2f", item.ManagementFee),
			fmt.Sprintf("%.2f", item.ResourceCostTotal),
			fmt.Sprintf("%.2f", item.TotalAmount),
			item.Remark,
			l.formatTimestamp(item.CreatedAt),
		}

		for colIdx, value := range rowData {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, row)
			f.SetCellValue(sheetName, cell, value)

			// 根据列和行应用不同样式
			var style int
			if colIdx == totalAmountColIndex {
				if isOddRow {
					style = totalStyleOdd
				} else {
					style = totalStyleEven
				}
			} else if moneyColIndices[colIdx] {
				if isOddRow {
					style = moneyStyleOdd
				} else {
					style = moneyStyleEven
				}
			} else {
				if isOddRow {
					style = oddRowStyle
				} else {
					style = evenRowStyle
				}
			}
			f.SetCellStyle(sheetName, cell, cell, style)
		}

		f.SetRowHeight(sheetName, row, 22)
	}

	// 设置列宽
	columnWidths := map[string]float64{
		"A": 22, "B": 12, "C": 18, "D": 18,
		"E": 20, "F": 20, "G": 14,
		"H": 10, "I": 12, "J": 12, "K": 10, "L": 10,
		"M": 10, "N": 10,
		"O": 10, "P": 10, "Q": 10, "R": 10, "S": 10, "T": 12,
		"U": 12, "V": 12, "W": 12, "X": 12, "Y": 12, "Z": 12,
		"AA": 12, "AB": 14, "AC": 20, "AD": 20,
	}
	for col, width := range columnWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// 冻结首行
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// 添加自动筛选
	lastDataRow := len(result.List) + 1
	lastColName, _ := excelize.CoordinatesToCellName(len(headers), lastDataRow)
	f.AutoFilter(sheetName, fmt.Sprintf("A1:%s", lastColName), nil)

	// 添加汇总工作表
	l.addSummarySheet(f, result.Summary)

	// 生成文件名
	fileName := fmt.Sprintf("账单明细_%s.xlsx", time.Now().Format("20060102150405"))

	// 设置响应头
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(fileName)))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if err := f.Write(w); err != nil {
		l.Errorf("写入Excel文件失败: %v", err)
		return fmt.Errorf("写入Excel文件失败: %v", err)
	}

	return nil
}

// 添加汇总工作表
func (l *OnecBillingStatementExportLogic) addSummarySheet(f *excelize.File, summary *pb.BillingStatementSummary) {
	sheetName := "汇总统计"
	f.NewSheet(sheetName)

	// 标题样式
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   16,
			Color:  "#4F46E5",
			Family: "Microsoft YaHei",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	// 表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   11,
			Color:  "#FFFFFF",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4F46E5"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#CBD5E1", Style: 1},
			{Type: "top", Color: "#CBD5E1", Style: 1},
			{Type: "bottom", Color: "#CBD5E1", Style: 1},
			{Type: "right", Color: "#CBD5E1", Style: 1},
		},
	})

	// 数据行样式
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:   11,
			Color:  "#1E293B",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#F8FAFC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 金额样式
	amountStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   12,
			Color:  "#059669",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#F8FAFC"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#E2E8F0", Style: 1},
			{Type: "top", Color: "#E2E8F0", Style: 1},
			{Type: "bottom", Color: "#E2E8F0", Style: 1},
			{Type: "right", Color: "#E2E8F0", Style: 1},
		},
	})

	// 总计样式
	totalLabelStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   12,
			Color:  "#FFFFFF",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#7C3AED"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#6D28D9", Style: 2},
			{Type: "top", Color: "#6D28D9", Style: 2},
			{Type: "bottom", Color: "#6D28D9", Style: 2},
			{Type: "right", Color: "#6D28D9", Style: 2},
		},
	})

	totalValueStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   14,
			Color:  "#FFFFFF",
			Family: "Microsoft YaHei",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#7C3AED"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#6D28D9", Style: 2},
			{Type: "top", Color: "#6D28D9", Style: 2},
			{Type: "bottom", Color: "#6D28D9", Style: 2},
			{Type: "right", Color: "#6D28D9", Style: 2},
		},
	})

	// 写入标题
	f.MergeCell(sheetName, "A1", "B1")
	f.SetCellValue(sheetName, "A1", "账单汇总统计")
	f.SetCellStyle(sheetName, "A1", "B1", titleStyle)
	f.SetRowHeight(sheetName, 1, 35)

	// 生成时间
	f.MergeCell(sheetName, "A2", "B2")
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("生成时间：%s", time.Now().Format("2006-01-02 15:04:05")))
	f.SetRowHeight(sheetName, 2, 22)

	// 表头
	f.SetCellValue(sheetName, "A4", "统计项")
	f.SetCellValue(sheetName, "B4", "金额/数量")
	f.SetCellStyle(sheetName, "A4", "B4", headerStyle)
	f.SetRowHeight(sheetName, 4, 28)

	// 汇总数据
	summaryData := [][]interface{}{
		{"账单总数", fmt.Sprintf("%d 条", summary.TotalCount)},
		{"CPU费用合计", fmt.Sprintf("¥%.2f", summary.CpuCost)},
		{"内存费用合计", fmt.Sprintf("¥%.2f", summary.MemoryCost)},
		{"存储费用合计", fmt.Sprintf("¥%.2f", summary.StorageCost)},
		{"GPU费用合计", fmt.Sprintf("¥%.2f", summary.GpuCost)},
		{"Pod费用合计", fmt.Sprintf("¥%.2f", summary.PodCost)},
		{"管理费合计", fmt.Sprintf("¥%.2f", summary.ManagementFee)},
	}

	for i, rowData := range summaryData {
		row := i + 5
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), rowData[0])
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), rowData[1])
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), dataStyle)
		f.SetCellStyle(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), amountStyle)
		f.SetRowHeight(sheetName, row, 24)
	}

	// 总费用行
	totalRow := len(summaryData) + 5
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", totalRow), "总费用")
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", totalRow), fmt.Sprintf("¥%.2f", summary.TotalAmount))
	f.SetCellStyle(sheetName, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("A%d", totalRow), totalLabelStyle)
	f.SetCellStyle(sheetName, fmt.Sprintf("B%d", totalRow), fmt.Sprintf("B%d", totalRow), totalValueStyle)
	f.SetRowHeight(sheetName, totalRow, 30)

	// 设置列宽
	f.SetColWidth(sheetName, "A", "A", 18)
	f.SetColWidth(sheetName, "B", "B", 20)
}

// 格式化时间戳
func (l *OnecBillingStatementExportLogic) formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

// 获取账单类型文本
func (l *OnecBillingStatementExportLogic) getStatementTypeText(statementType string) string {
	switch statementType {
	case "daily":
		return "日账单"
	case "config_change":
		return "配置变更"
	case "initial":
		return "初始账单"
	default:
		return statementType
	}
}
