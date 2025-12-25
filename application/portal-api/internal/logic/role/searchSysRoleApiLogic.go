package role

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysRoleApiLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysRoleApiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysRoleApiLogic {
	return &SearchSysRoleApiLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysRoleApiLogic) SearchSysRoleApi(req *types.SearchSysRoleApiRequest) (resp []uint64, err error) {

	// 调用 RPC 服务查询角色API权限
	res, err := l.svcCtx.PortalRpc.RoleSearchApi(l.ctx, &pb.SearchSysRoleApiReq{
		RoleId: req.RoleId,
	})
	if err != nil {
		l.Errorf("查询角色API权限失败: roleId=%d, error=%v", req.RoleId, err)
		return nil, err
	}

	return res.ApiIds, nil
}
