package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateJobParallelismLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateJobParallelismLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateJobParallelismLogic {
	return &UpdateJobParallelismLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateJobParallelismLogic) UpdateJobParallelism(req *types.UpdateJobParallelismRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取原并行度配置
	oldConfig, err := client.Job().GetParallelismConfig(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取原并行度配置失败: %v", err)
		// 继续执行
	}

	updateReq := &k8sTypes.UpdateJobParallelismRequest{
		Name:                  versionDetail.ResourceName,
		Namespace:             versionDetail.Namespace,
		Parallelism:           &req.Parallelism,
		Completions:           &req.Completions,
		BackoffLimit:          &req.BackoffLimit,
		ActiveDeadlineSeconds: &req.ActiveDeadlineSeconds,
	}

	err = client.Job().UpdateParallelismConfig(updateReq)

	// 生成变更详情
	var changeDetail string
	if oldConfig != nil {
		changeDetail = CompareJobParallelism(oldConfig, &req.Parallelism, &req.Completions, &req.BackoffLimit, &req.ActiveDeadlineSeconds)
	} else {
		changeDetail = fmt.Sprintf("Job 并行度配置变更 (无法获取原配置): 并行度: %d, 完成数: %d, 重试次数: %d, 超时时间: %ds",
			req.Parallelism, req.Completions, req.BackoffLimit, req.ActiveDeadlineSeconds)
	}

	if err != nil {
		l.Errorf("修改 Job 并行度配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改并行度配置",
			fmt.Sprintf("Job %s/%s 修改并行度配置失败, %s, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, changeDetail, err), 2)
		return "", fmt.Errorf("修改 Job 并行度配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改并行度配置",
		fmt.Sprintf("Job %s/%s 修改并行度配置成功, %s", versionDetail.Namespace, versionDetail.ResourceName, changeDetail), 1)
	return "修改 Job 并行度配置成功", nil
}
