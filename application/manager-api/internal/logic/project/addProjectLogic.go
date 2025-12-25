package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建新项目
func NewAddProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectLogic {
	return &AddProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}
func (l *AddProjectLogic) AddProject(req *types.AddProjectRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务创建项目
	_, err = l.svcCtx.ManagerRpc.ProjectAdd(l.ctx, &pb.AddOnecProjectReq{
		Name:        req.Name,
		IsSystem:    req.IsSystem,
		Description: req.Description,
		CreatedBy:   username,
		UpdatedBy:   username,
	})

	if err != nil {
		l.Errorf("创建项目失败: %v", err)
		return "", fmt.Errorf("创建项目失败: %v", err)
	}

	return "项目创建成功", nil
}
