package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysRoleByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysRoleByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysRoleByIdLogic {
	return &GetSysRoleByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysRoleByIdLogic) GetSysRoleById(req *types.DefaultIdRequest) (resp *types.SysRole, err error) {

	// 调用 RPC 服务获取角色信息
	res, err := l.svcCtx.PortalRpc.RoleGetById(l.ctx, &pb.GetSysRoleByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取角色失败: roleId=%d, error=%v", req.Id, err)
		return nil, err
	}

	return &types.SysRole{
		Id:        res.Data.Id,
		Name:      res.Data.Name,
		Code:      res.Data.Code,
		Remark:    res.Data.Remark,
		CreatedBy: res.Data.CreatedBy,
		UpdatedBy: res.Data.UpdatedBy,
		CreatedAt: res.Data.CreatedAt,
		UpdatedAt: res.Data.UpdatedAt,
	}, nil
}
