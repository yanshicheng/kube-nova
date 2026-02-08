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

type SearchSysPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysPlatformLogic {
	return &SearchSysPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysPlatformLogic) SearchSysPlatform(req *types.SearchSysPlatformRequest) (resp *types.SearchSysPlatformResponse, err error) {
	// 调用 RPC 服务搜索平台
	rpcResp, err := l.svcCtx.PortalRpc.PlatformSearch(l.ctx, &pb.SearchSysPlatformReq{
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderField:   req.OrderStr,
		IsAsc:        req.IsAsc,
		PlatformCode: req.PlatformCode,
		PlatformName: req.PlatformName,
		IsEnable:     req.IsEnable,
	})
	if err != nil {
		l.Errorf("搜索平台失败: error=%v", err)
		return nil, err
	}

	// 转换为 API 响应格式
	var platforms []types.SysPlatform
	for _, platform := range rpcResp.Data {
		platforms = append(platforms, types.SysPlatform{
			Id:           platform.Id,
			PlatformCode: platform.PlatformCode,
			PlatformName: platform.PlatformName,
			PlatformDesc: platform.PlatformDesc,
			PlatformIcon: platform.PlatformIcon,
			Sort:         platform.Sort,
			IsEnable:     platform.IsEnable,
			IsDefault:    platform.IsDefault,
			CreatedBy:    platform.CreateBy,
			UpdatedBy:    platform.UpdateBy,
			CreatedAt:    platform.CreateTime,
			UpdatedAt:    platform.UpdateTime,
		})
	}

	return &types.SearchSysPlatformResponse{
		Items: platforms,
		Total: rpcResp.Total,
	}, nil
}
