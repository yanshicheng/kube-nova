package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取GC详情
func NewGetGCLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCLogic {
	return &GetGCLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGCLogic) GetGC(req *types.GetGCRequest) (resp *types.GetGCResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetGC(l.ctx, &pb.GetGCReq{
		RegistryUuid: req.RegistryUuid,
		GcId:         req.GcId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	l.Infof("GC详情查询成功: GcId=%d", req.GcId)
	return &types.GetGCResponse{
		Data: types.GCHistory{
			Id:             rpcResp.Data.Id,
			JobName:        rpcResp.Data.JobName,
			JobKind:        rpcResp.Data.JobKind,
			JobStatus:      rpcResp.Data.JobStatus,
			CreationTime:   rpcResp.Data.CreationTime,
			UpdateTime:     rpcResp.Data.UpdateTime,
			DeleteUntagged: rpcResp.Data.DeleteUntagged,
			JobParameters:  rpcResp.Data.JobParameters,
		},
	}, nil
}
