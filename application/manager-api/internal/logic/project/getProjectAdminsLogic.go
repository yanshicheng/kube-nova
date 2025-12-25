package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
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

	// 调用RPC服务获取项目管理员列表
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectAdminGetByProjectId(l.ctx, &pb.GetOnecProjectAdminsByProjectIdReq{
		ProjectId: req.ProjectId,
	})

	if err != nil {
		l.Errorf("查询项目管理员列表失败: %v", err)
		return nil, fmt.Errorf("查询项目管理员列表失败: %v", err)
	}

	// 转换响应数据
	admins := make([]types.ProjectAdmin, 0, len(rpcResp.Data))
	for _, admin := range rpcResp.Data {
		admins = append(admins, types.ProjectAdmin{
			Id:        admin.Id,
			ProjectId: admin.ProjectId,
			UserId:    admin.UserId,
			CreatedAt: admin.CreatedAt,
		})
	}
	return admins, nil
}
