package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPauseStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPauseStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPauseStatusLogic {
	return &GetPauseStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPauseStatusLogic) GetPauseStatus(req *types.DefaultIdRequest) (resp *types.PauseStatusResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resp = &types.PauseStatusResponse{}

	switch strings.ToUpper(versionDetail.ResourceType) {
	case "DEPLOYMENT":
		status, err := client.Deployment().GetPauseStatus(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 暂停状态失败: %v", err)
			return nil, fmt.Errorf("获取暂停状态失败")
		}
		resp.Paused = status.Paused
		resp.SupportType = "pause"
	case "JOB":
		status, err := client.Job().GetPauseStatus(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Job 暂停状态失败: %v", err)
			return nil, fmt.Errorf("获取暂停状态失败")
		}
		resp.Paused = status.Paused
		resp.SupportType = "suspend"
	case "CRONJOB":
		status, err := client.CronJob().GetPauseStatus(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 CronJob 暂停状态失败: %v", err)
			return nil, fmt.Errorf("获取暂停状态失败")
		}
		resp.Paused = status.Paused
		resp.SupportType = "suspend"
	default:
		resp.Paused = false
		resp.SupportType = "none"
	}

	return resp, nil
}
