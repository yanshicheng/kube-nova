package token

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysTokenByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysTokenByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysTokenByIdLogic {
	return &GetSysTokenByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysTokenByIdLogic) GetSysTokenById(req *types.DefaultIdRequest) (resp *types.SysToken, err error) {

	// 调用 RPC 服务获取Token信息
	res, err := l.svcCtx.PortalRpc.TokenGetById(l.ctx, &pb.GetSysTokenByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取Token失败: tokenId=%d, error=%v", req.Id, err)
		return nil, err
	}

	return &types.SysToken{
		Id:         res.Data.Id,
		OwnerType:  res.Data.OwnerType,
		OwnerId:    res.Data.OwnerId,
		Token:      res.Data.Token,
		Name:       res.Data.Name,
		Type:       res.Data.Type,
		ExpireTime: res.Data.ExpireTime,
		Status:     res.Data.Status,
		CreatedBy:  res.Data.CreatedBy,
		UpdatedBy:  res.Data.UpdatedBy,
		CreatedAt:  res.Data.CreatedAt,
		UpdatedAt:  res.Data.UpdatedAt,
	}, nil
}
