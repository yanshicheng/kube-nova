// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformUsersLogic {
	return &GetPlatformUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformUsersLogic) GetPlatformUsers(req *types.GetPlatformUsersRequest) (resp *types.GetPlatformUsersResponse, err error) {
	// 调用 RPC 服务获取平台用户列表
	rpcResp, err := l.svcCtx.PortalRpc.PlatformUsersGet(l.ctx, &pb.GetPlatformUsersReq{
		PlatformId: req.PlatformId,
		Page:       req.Page,
		PageSize:   req.PageSize,
	})
	if err != nil {
		l.Errorf("获取平台用户列表失败: platformId=%d, error=%v", req.PlatformId, err)
		return nil, err
	}

	// 转换为 API 响应格式
	return &types.GetPlatformUsersResponse{
		UserIds: rpcResp.UserIds,
		Total:   rpcResp.Total,
	}, nil
}
