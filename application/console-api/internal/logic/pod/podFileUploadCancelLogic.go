package pod

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadCancelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPodFileUploadCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileUploadCancelLogic {
	return &PodFileUploadCancelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileUploadCancelLogic) PodFileUploadCancel(req *types.PodFileUploadCancelReq) (resp *types.PodFileUploadCancelResp, err error) {
	// 通过 ServiceContext 调用通用上传服务
	err = l.svcCtx.UploadCoreClient.Cancel(req.SessionId)

	if err != nil {
		return &types.PodFileUploadCancelResp{
			SessionId: req.SessionId,
			Canceled:  false,
			Message:   err.Error(),
		}, nil
	}

	return &types.PodFileUploadCancelResp{
		SessionId: req.SessionId,
		Canceled:  true,
		Message:   "上传已取消",
	}, nil
}
