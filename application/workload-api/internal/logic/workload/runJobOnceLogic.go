package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunJobOnceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRunJobOnceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunJobOnceLogic {
	return &RunJobOnceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RunJobOnceLogic) RunJobOnce(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	job, err := client.Job().Recreate(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("手动运行 Job 失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "手动运行Job",
			fmt.Sprintf("Job %s/%s 手动运行失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
		return "", fmt.Errorf("手动运行 Job 失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "手动运行Job",
		fmt.Sprintf("Job %s/%s 手动运行成功, 新创建Job名称: %s", versionDetail.Namespace, versionDetail.ResourceName, job.Name), 1)
	return fmt.Sprintf("成功创建新 Job: %s", job.Name), nil
}
