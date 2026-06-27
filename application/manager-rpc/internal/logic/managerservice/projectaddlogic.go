package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAddLogic {
	return &ProjectAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAdd 创建项目（委托 portal-rpc）
func (l *ProjectAddLogic) ProjectAdd(in *pb.AddOnecProjectReq) (*pb.AddOnecProjectResp, error) {
	if in.Name == "" || in.CreatedBy == "" {
		l.Errorf("参数错误: %v", in)
		return nil, errorx.Msg("参数错误")
	}

	_, err := l.svcCtx.PortalProjectRpc.CreateProject(l.ctx, &portalpb.PortalCreateProjectReq{
		Name:        in.Name,
		IsSystem:    in.IsSystem,
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		l.Errorf("调用 portal 创建项目失败: %v", err)
		return nil, errorx.Msg("创建项目失败")
	}

	return &pb.AddOnecProjectResp{}, nil
}
