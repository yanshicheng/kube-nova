package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGCLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCLogLogic {
	return &GetGCLogLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGCLogLogic) GetGCLog(in *pb.GetGCLogReq) (*pb.GetGCLogResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	log, err := client.GC().GetLog(in.GcId)
	if err != nil {
		return nil, errorx.Msg("获取GC日志失败")
	}

	return &pb.GetGCLogResp{
		Log: log,
	}, nil
}
