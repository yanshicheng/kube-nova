package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索文件
func NewPodFileSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileSearchLogic {
	return &PodFileSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileSearchLogic) PodFileSearch(req *types.PodFileSearchReq) (resp *types.PodFileSearchResp, err error) {
	// 1. 获取集群和命名空间信息

	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
		l.Infof("使用默认容器: %s", containerName)
	}

	// 5. 设置默认搜索路径
	searchPath := req.Path
	if searchPath == "" {
		searchPath = "/"
	}

	var modifiedAfter, modifiedBefore *time.Time
	if req.ModifiedAfter != "" {
		t, err := time.Parse(time.RFC3339, req.ModifiedAfter)
		if err != nil {
			l.Errorf("解析修改时间失败: %v", err)
			return nil, fmt.Errorf("修改时间格式错误，请使用 RFC3339 格式（例如：2024-01-01T00:00:00Z）")
		}
		modifiedAfter = &t
		l.Debugf("设置修改时间下限: %v", t)
	}
	if req.ModifiedBefore != "" {
		t, err := time.Parse(time.RFC3339, req.ModifiedBefore)
		if err != nil {
			l.Errorf("解析修改时间失败: %v", err)
			return nil, fmt.Errorf("修改时间格式错误，请使用 RFC3339 格式（例如：2024-01-01T00:00:00Z）")
		}
		modifiedBefore = &t
		l.Debugf("设置修改时间上限: %v", t)
	}

	// 7. 构建搜索选项
	searchOpts := &k8stypes.FileSearchOptions{
		Pattern:        req.Pattern,
		ContentSearch:  req.ContentSearch,
		FileTypes:      req.FileTypes,
		MinSize:        req.MinSize,
		MaxSize:        req.MaxSize,
		ModifiedAfter:  modifiedAfter,
		ModifiedBefore: modifiedBefore,
		MaxDepth:       req.MaxDepth,
		MaxResults:     req.MaxResults,
		CaseSensitive:  req.CaseSensitive,
		FollowLinks:    false, // 默认不跟随符号链接
	}

	// 设置默认值
	if searchOpts.MaxResults == 0 {
		searchOpts.MaxResults = 100
		l.Debug("使用默认最大结果数: 100")
	}
	if searchOpts.MaxDepth == 0 {
		searchOpts.MaxDepth = 10
		l.Debug("使用默认最大深度: 10")
	}

	l.Infof("开始搜索文件: namespace=%s, pod=%s, container=%s, path=%s, pattern=%s, contentSearch=%s, maxResults=%d, maxDepth=%d",
		workloadInfo.Data.Namespace, req.PodName, containerName, searchPath,
		req.Pattern, req.ContentSearch, searchOpts.MaxResults, searchOpts.MaxDepth)

	// 8. 执行搜索
	searchResult, err := podClient.SearchFiles(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, searchPath, searchOpts)
	if err != nil {
		l.Errorf("搜索文件失败: %v", err)
		return nil, fmt.Errorf("搜索文件失败: %v", err)
	}

	// 9. 转换搜索结果
	results := make([]types.FileSearchResult, 0, len(searchResult.Results))
	for _, r := range searchResult.Results {
		// 转换内容匹配
		matches := make([]types.ContentMatch, 0, len(r.Matches))
		for _, m := range r.Matches {
			matches = append(matches, types.ContentMatch{
				LineNumber: m.LineNumber,
				Line:       m.Line,
				Preview:    m.Preview,
			})
		}

		// 格式化修改时间为 RFC3339
		modTimeStr := r.ModTime.Format(time.RFC3339)

		results = append(results, types.FileSearchResult{
			Path:    r.Path,
			Name:    r.Name,
			Size:    r.Size,
			ModTime: modTimeStr,
			IsDir:   r.IsDir,
			Matches: matches,
		})
	}

	// 10. 构建响应
	resp = &types.PodFileSearchResp{
		Results:    results,
		TotalFound: searchResult.TotalFound,
		SearchTime: searchResult.SearchTime,
		Truncated:  searchResult.Truncated,
	}

	// 详细的日志信息
	if req.ContentSearch != "" {
		l.Infof("文件搜索完成 - 模式: %s, 内容搜索: %s, 找到: %d 个结果, 耗时: %dms, 截断: %v",
			req.Pattern, req.ContentSearch, resp.TotalFound, resp.SearchTime, resp.Truncated)
	} else {
		l.Infof("文件搜索完成 - 模式: %s, 找到: %d 个结果, 耗时: %dms, 截断: %v",
			req.Pattern, resp.TotalFound, resp.SearchTime, resp.Truncated)
	}

	return resp, nil
}
