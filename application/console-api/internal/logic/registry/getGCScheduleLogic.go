package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取GC调度配置
func NewGetGCScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCScheduleLogic {
	return &GetGCScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGCScheduleLogic) GetGCSchedule(req *types.GetGCScheduleRequest) (resp *types.GetGCScheduleResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetGCSchedule(l.ctx, &pb.GetGCScheduleReq{
		RegistryUuid: req.RegistryUuid,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	l.Infof("GC调度配置查询成功")
	return &types.GetGCScheduleResponse{
		Data: types.GCSchedule{
			Schedule:   rpcResp.Data.Schedule,
			Parameters: rpcResp.Data.Parameters,
		},
	}, nil
}
