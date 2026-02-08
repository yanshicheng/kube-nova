package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectRegistryBindingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询应用项目的仓库项目绑定
func NewListProjectRegistryBindingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectRegistryBindingsLogic {
	return &ListProjectRegistryBindingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProjectRegistryBindingsLogic) ListProjectRegistryBindings(req *types.ListProjectRegistryBindingsRequest) (resp *types.ListProjectRegistryBindingsResponse, err error) {
	// 调用 RPC
	rpcResp, err := l.svcCtx.RepositoryRpc.ListProjectRegistryBindings(l.ctx, &pb.ListProjectRegistryBindingsReq{
		RegistryId:          req.RegistryId,
		RegistryProjectName: req.RegistryProjectName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}
	if rpcResp.Data == nil {
		return &types.ListProjectRegistryBindingsResponse{}, nil
	}
	var data []types.BindProjectIds
	if rpcResp.Data != nil {
		data = make([]types.BindProjectIds, 0, len(rpcResp.Data))
		for _, item := range rpcResp.Data {
			data = append(data, types.BindProjectIds{
				Id:        item.Id,
				ProjectId: item.ProjectId,
			})
		}
	}

	return &types.ListProjectRegistryBindingsResponse{
		Data: data,
	}, nil
}
