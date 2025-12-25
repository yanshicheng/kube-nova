package operator

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GCOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewGCOperator(ctx context.Context, base *BaseOperator) types.GCOperator {
	return &GCOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

// ListHistory 列出 GC 历史
func (g *GCOperatorImpl) ListHistory(req types.ListRequest) (*types.ListGCHistoryResponse, error) {
	g.log.Infof("列出 GC 历史: page=%d, pageSize=%d", req.Page, req.PageSize)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 获取所有 GC 历史
	var allGCs []types.GCHistory
	page := 1

	for {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page_size": "100",
		}

		query := g.buildQuery(params)
		path := "/api/v2.0/system/gc" + query

		var gcs []types.GCHistory
		if err := g.doRequest("GET", path, nil, &gcs); err != nil {
			return nil, err
		}

		if len(gcs) == 0 {
			break
		}

		allGCs = append(allGCs, gcs...)

		if len(gcs) < 100 {
			break
		}
		page++
	}

	g.log.Infof("获取到 %d 条 GC 历史", len(allGCs))

	// 应用搜索过滤
	if req.Search != "" {
		filtered := make([]types.GCHistory, 0)
		searchLower := strings.ToLower(req.Search)

		for _, gc := range allGCs {
			if strings.Contains(strings.ToLower(gc.JobName), searchLower) ||
				strings.Contains(strings.ToLower(gc.JobStatus), searchLower) {
				filtered = append(filtered, gc)
			}
		}
		allGCs = filtered
		g.log.Infof("搜索过滤后剩余 %d 条记录", len(allGCs))
	}

	// 排序 - 按创建时间倒序
	sort.Slice(allGCs, func(i, j int) bool {
		return allGCs[i].CreationTime.After(allGCs[j].CreationTime)
	})

	// 计算分页
	total := len(allGCs)
	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	start := (int(req.Page) - 1) * int(req.PageSize)
	end := start + int(req.PageSize)

	if start >= total {
		return &types.ListGCHistoryResponse{
			ListResponse: types.ListResponse{
				Total:      total,
				Page:       req.Page,
				PageSize:   req.PageSize,
				TotalPages: totalPages,
			},
			Items: []types.GCHistory{},
		}, nil
	}

	if end > total {
		end = total
	}

	pageGCs := allGCs[start:end]

	g.log.Infof("返回 GC 历史: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(pageGCs))

	return &types.ListGCHistoryResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: pageGCs,
	}, nil
}

// Get 获取 GC 详情
func (g *GCOperatorImpl) Get(gcID int64) (*types.GCHistory, error) {
	g.log.Infof("获取 GC 详情: gcID=%d", gcID)

	var gc types.GCHistory
	path := fmt.Sprintf("/api/v2.0/system/gc/%d", gcID)

	if err := g.doRequest("GET", path, nil, &gc); err != nil {
		return nil, err
	}

	g.log.Infof("获取 GC 详情成功: status=%s", gc.JobStatus)
	return &gc, nil
}

// GetLog 获取 GC 日志
// GetLog 获取 GC 日志
// GetLog 获取 GC 日志
func (g *GCOperatorImpl) GetLog(gcID int64) (string, error) {
	g.log.Infof("获取 GC 日志: gcID=%d", gcID)

	path := fmt.Sprintf("/api/v2.0/system/gc/%d/log", gcID)

	// 日志是纯文本格式，需要特殊处理
	resp, err := g.doRequestWithHeaders("GET", path, nil, map[string]string{
		"Accept": "text/plain",
	})
	if err != nil {
		return "", err
	}

	g.log.Infof("获取 GC 日志成功: 长度=%d", len(resp))
	return string(resp), nil
}

// GetSchedule 获取 GC 调度配置
func (g *GCOperatorImpl) GetSchedule() (*types.GCSchedule, error) {
	g.log.Info("获取 GC 调度配置")

	var schedule types.GCSchedule
	path := "/api/v2.0/system/gc/schedule"

	if err := g.doRequest("GET", path, nil, &schedule); err != nil {
		return nil, err
	}

	g.log.Infof("获取 GC 调度配置成功: schedule=%v", schedule.Schedule)
	return &schedule, nil
}

// UpdateSchedule 更新 GC 调度配置
func (g *GCOperatorImpl) UpdateSchedule(schedule string, deleteUntagged bool) error {
	g.log.Infof("更新 GC 调度配置: schedule=%s, deleteUntagged=%v", schedule, deleteUntagged)

	// 构建调度配置
	scheduleConfig := &types.GCSchedule{
		Parameters: map[string]string{
			"delete_untagged": strconv.FormatBool(deleteUntagged),
		},
	}

	// 如果提供了 cron 表达式，设置调度
	if schedule != "" {
		scheduleConfig.Schedule = &types.Schedule{
			Type: "Custom",
			Cron: schedule,
		}
	} else {
		// 空字符串表示禁用调度
		scheduleConfig.Schedule = nil
	}

	path := "/api/v2.0/system/gc/schedule"

	if err := g.doRequest("PUT", path, scheduleConfig, nil); err != nil {
		return err
	}

	g.log.Info("更新 GC 调度配置成功")
	return nil
}

// Trigger 手动触发 GC
func (g *GCOperatorImpl) Trigger(deleteUntagged bool, workers int) (int64, error) {
	g.log.Infof("手动触发 GC: deleteUntagged=%v, workers=%d", deleteUntagged, workers)

	// 构建 GC 参数
	body := map[string]interface{}{
		"schedule": map[string]interface{}{
			"type": "Manual",
		},
		"parameters": map[string]interface{}{
			"delete_untagged": deleteUntagged,
		},
	}

	// 如果指定了 workers，添加到参数中
	if workers > 0 {
		body["parameters"].(map[string]interface{})["workers"] = workers
	}

	path := "/api/v2.0/system/gc/schedule"

	// Harbor 2.0 API 返回的是 Location header 中的 job ID
	// 但我们这里简化处理，返回 0 表示触发成功
	if err := g.doRequest("POST", path, body, nil); err != nil {
		return 0, err
	}

	g.log.Info("触发 GC 成功")

	// 获取最新的 GC 记录
	history, err := g.ListHistory(types.ListRequest{
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		g.log.Errorf("获取最新 GC 记录失败: %v", err)
		return 0, nil
	}

	if len(history.Items) > 0 {
		return history.Items[0].ID, nil
	}

	return 0, nil
}
