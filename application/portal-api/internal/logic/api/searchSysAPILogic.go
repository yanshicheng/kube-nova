package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysAPILogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysAPILogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysAPILogic {
	return &SearchSysAPILogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysAPILogic) SearchSysAPI(req *types.SearchSysAPIRequest) (resp *types.SearchSysAPIResponse, err error) {

	// 调用 RPC 服务搜索API
	res, err := l.svcCtx.PortalRpc.APISearch(l.ctx, &pb.SearchSysAPIReq{
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderField:   req.OrderStr,
		IsAsc:        req.IsAsc,
		ParentId:     req.ParentId,
		Name:         req.Name,
		Path:         req.Path,
		Method:       req.Method,
		IsPermission: req.IsPermission,
	})
	if err != nil {
		l.Errorf("搜索API失败: error=%v", err)
		return nil, err
	}

	// 转换API列表
	var apis []types.SysAPI
	for _, api := range res.Data {
		apis = append(apis, types.SysAPI{
			Id:           api.Id,
			ParentId:     api.ParentId,
			Name:         api.Name,
			Path:         api.Path,
			Method:       api.Method,
			IsPermission: api.IsPermission,
			CreatedBy:    api.CreatedBy,
			UpdatedBy:    api.UpdatedBy,
			CreatedAt:    api.CreatedAt,
			UpdatedAt:    api.UpdatedAt,
		})
	}

	return &types.SearchSysAPIResponse{
		Items: apis,
		Total: res.Total,
	}, nil
}
