package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TriggerGCLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTriggerGCLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TriggerGCLogic {
	return &TriggerGCLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TriggerGCLogic) TriggerGC(in *pb.TriggerGCReq) (*pb.TriggerGCResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	workers := int(in.Workers)
	if workers <= 0 {
		workers = 1
	}

	gcID, err := client.GC().Trigger(in.DeleteUntagged, workers)
	if err != nil {
		return nil, errorx.Msg("触发GC失败")
	}

	return &pb.TriggerGCResp{
		JobId:   gcID,
		Message: "GC任务已触发",
	}, nil
}
