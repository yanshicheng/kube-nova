package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectsByUserIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据用户ID查询项目列表
func NewGetProjectsByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectsByUserIdLogic {
	return &GetProjectsByUserIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectsByUserIdLogic) GetProjectsByUserId(req *types.GetProjectsByUserIdRequest) (resp []types.Project, err error) {
	userId, ok := l.ctx.Value("userId").(uint64)
	if !ok || userId == 0 {
		userId = 0
	}
	userRoles, ok := l.ctx.Value("roles").([]string)
	if !ok {
		userRoles = []string{}
	}
	projects, err := l.svcCtx.ManagerRpc.ProjectGetByUserId(l.ctx, &managerservice.GetOnecProjectsByUserIdReq{
		UserId: userId,
		Name:   req.Name,
		Roles:  userRoles,
	})
	if err != nil {
		l.Errorf("查询用户 [ID:%d] 的项目列表失败: %v", userId, err)
		return nil, errorx.Msg("查询用户项目列表失败")
	}
	var data []types.Project
	for _, project := range projects.Data {
		data = append(data, types.Project{
			Id:       project.Id,
			Name:     project.Name,
			Uuid:     project.Uuid,
			IsSystem: project.IsSystem,
		})
	}

	return data, nil
}
