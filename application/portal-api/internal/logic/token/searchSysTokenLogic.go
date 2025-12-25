package token

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysTokenLogic {
	return &SearchSysTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysTokenLogic) SearchSysToken(req *types.SearchSysTokenRequest) (resp *types.SearchSysTokenResponse, err error) {

	// 调用 RPC 服务搜索Token
	res, err := l.svcCtx.PortalRpc.TokenSearch(l.ctx, &pb.SearchSysTokenReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderStr,
		IsAsc:      req.IsAsc,
		OwnerType:  req.OwnerType,
		OwnerId:    req.OwnerId,
		Token:      req.Token,
		Name:       req.Name,
		Type:       req.Type,
	})
	if err != nil {
		l.Errorf("搜索Token失败: error=%v", err)
		return nil, err
	}

	// 转换Token列表
	var tokens []types.SysToken
	for _, token := range res.Data {
		tokens = append(tokens, types.SysToken{
			Id:         token.Id,
			OwnerType:  token.OwnerType,
			OwnerId:    token.OwnerId,
			Token:      token.Token,
			Name:       token.Name,
			Type:       token.Type,
			ExpireTime: token.ExpireTime,
			Status:     token.Status,
			CreatedBy:  token.CreatedBy,
			UpdatedBy:  token.UpdatedBy,
			CreatedAt:  token.CreatedAt,
			UpdatedAt:  token.UpdatedAt,
		})
	}

	return &types.SearchSysTokenResponse{
		Items: tokens,
		Total: res.Total,
	}, nil
}
