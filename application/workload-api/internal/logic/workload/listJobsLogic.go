package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListJobsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取Job列表（带分页）
func NewListJobsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListJobsLogic {
	return &ListJobsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListJobsLogic) ListJobs(req *types.ListJobsRequest) (resp *types.ListJobsResponse, err error) {
	workspace, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.Id})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}
	jobs, err := client.Job().List(workspace.Data.Namespace, types2.ListRequest{
		Search:   req.Search,
		Page:     req.Page,
		PageSize: req.PageSize,
		SortBy:   req.SortBy,
		SortDesc: req.SortDesc,
		Labels:   req.Labels,
	})

	// 4. 转换数据结构
	resp = &types.ListJobsResponse{
		Total:      jobs.Total,
		Page:       jobs.Page,
		PageSize:   jobs.PageSize,
		TotalPages: jobs.TotalPages,
		Items:      make([]types.JobInfo, 0, len(jobs.Items)),
	}

	// 5. 转换 Job 详情
	for _, job := range jobs.Items {
		jobInfo := types.JobInfo{
			Name:              job.Name,
			Namespace:         job.Namespace,
			Completions:       job.Completions,
			Parallelism:       job.Parallelism,
			Succeeded:         job.Succeeded,
			Failed:            job.Failed,
			Active:            job.Active,
			Duration:          job.Duration,
			Status:            job.Status,
			CreationTimestamp: job.CreationTimestamp.Unix(), // 转换为秒时间戳
		}

		// 处理可选的时间字段
		if job.StartTime != nil {
			startTime := job.StartTime.Unix()
			jobInfo.StartTime = startTime
		}

		if job.CompletionTime != nil {
			completionTime := job.CompletionTime.Unix()
			jobInfo.CompletionTime = completionTime
		}

		resp.Items = append(resp.Items, jobInfo)
	}

	l.Infof("成功获取 Job 列表，总数: %d, 当前页: %d/%d", resp.Total, resp.Page, resp.TotalPages)
	return resp, nil
}
