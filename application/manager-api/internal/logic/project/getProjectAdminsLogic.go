package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectAdminsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetProjectAdminsLogic 根据项目ID获取所有管理员列表
func NewGetProjectAdminsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectAdminsLogic {
	return &GetProjectAdminsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectAdminsLogic) GetProjectAdmins(req *types.GetProjectAdminsRequest) (resp []types.ProjectAdmin, err error) {
	rpcResp, err := l.svcCtx.ProjectRpc.ListProjectMembers(l.ctx, &pb.PortalListProjectMembersReq{
		ProjectId: req.ProjectId,
	})

	if err != nil {
		l.Errorf("查询项目管理员列表失败: %v", err)
		return nil, err
	}

	admins := make([]types.ProjectAdmin, 0, len(rpcResp.Data))
	for _, admin := range rpcResp.Data {
		if admin.Role != "owner" && admin.Role != "admin" {
			continue
		}
		admins = append(admins, types.ProjectAdmin{
			Id:        admin.Id,
			ProjectId: admin.ProjectId,
			UserId:    admin.UserId,
			CreatedAt: admin.CreatedAt,
		})
	}
	return admins, nil
}
