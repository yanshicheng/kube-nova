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

type GetDefaultSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDefaultSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDefaultSysPlatformLogic {
	return &GetDefaultSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDefaultSysPlatformLogic) GetDefaultSysPlatform() (resp *types.SysPlatform, err error) {
	// 调用 RPC 服务获取默认平台
	rpcResp, err := l.svcCtx.PortalRpc.PlatformGetDefault(l.ctx, &pb.GetDefaultSysPlatformReq{})
	if err != nil {
		l.Errorf("获取默认平台失败: error=%v", err)
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
