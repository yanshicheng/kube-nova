package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取GC日志
func NewGetGCLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCLogLogic {
	return &GetGCLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGCLogLogic) GetGCLog(req *types.GetGCLogRequest) (resp *types.GetGCLogResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetGCLog(l.ctx, &pb.GetGCLogReq{
		RegistryUuid: req.RegistryUuid,
		GcId:         req.GcId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	l.Infof("GC日志查询成功: GcId=%d", req.GcId)
	return &types.GetGCLogResponse{
		Log: rpcResp.Log,
	}, nil
}
