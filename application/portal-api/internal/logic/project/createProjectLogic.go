package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProjectLogic {
	return &CreateProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateProjectLogic) CreateProject(req *types.PortalCreateProjectReq) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}

	_, err = l.svcCtx.ProjectRpc.CreateProject(l.ctx, &pb.PortalCreateProjectReq{
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    req.IsSystem,
	})
	if err != nil {
		l.Errorf("创建项目失败: %v", err)
		return "", err
	}

	return "项目创建成功", nil
}
