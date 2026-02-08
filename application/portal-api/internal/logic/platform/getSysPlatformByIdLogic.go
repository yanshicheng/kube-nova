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

type GetSysPlatformByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysPlatformByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysPlatformByIdLogic {
	return &GetSysPlatformByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysPlatformByIdLogic) GetSysPlatformById(req *types.DefaultIdRequest) (resp *types.SysPlatform, err error) {
	// 调用 RPC 服务获取平台详情
	rpcResp, err := l.svcCtx.PortalRpc.PlatformGetById(l.ctx, &pb.GetSysPlatformByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取平台详情失败: platformId=%d, error=%v", req.Id, err)
		return nil, err
	}

	// 转换为 API 响应格式
	return &types.SysPlatform{
		Id:           rpcResp.Data.Id,
		PlatformCode: rpcResp.Data.PlatformCode,
		PlatformName: rpcResp.Data.PlatformName,
		PlatformDesc: rpcResp.Data.PlatformDesc,
		PlatformIcon: rpcResp.Data.PlatformIcon,
		Sort:         rpcResp.Data.Sort,
		IsEnable:     rpcResp.Data.IsEnable,
		IsDefault:    rpcResp.Data.IsDefault,
		CreatedBy:    rpcResp.Data.CreateBy,
		UpdatedBy:    rpcResp.Data.UpdateBy,
		CreatedAt:    rpcResp.Data.CreateTime,
		UpdatedAt:    rpcResp.Data.UpdateTime,
	}, nil
}
