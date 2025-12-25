package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysAPIByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysAPIByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysAPIByIdLogic {
	return &GetSysAPIByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysAPIByIdLogic) GetSysAPIById(req *types.DefaultIdRequest) (resp *types.SysAPI, err error) {
	// 调用 RPC 服务获取API信息
	res, err := l.svcCtx.PortalRpc.APIGetById(l.ctx, &pb.GetSysAPIByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取API失败: apiId=%d, error=%v", req.Id, err)
		return nil, err
	}

	return &types.SysAPI{
		Id:           res.Data.Id,
		ParentId:     res.Data.ParentId,
		Name:         res.Data.Name,
		Path:         res.Data.Path,
		Method:       res.Data.Method,
		IsPermission: res.Data.IsPermission,
		CreatedBy:    res.Data.CreatedBy,
		UpdatedBy:    res.Data.UpdatedBy,
		CreatedAt:    res.Data.CreatedAt,
		UpdatedAt:    res.Data.UpdatedAt,
	}, nil
}
