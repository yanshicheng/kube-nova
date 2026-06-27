package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectUpdateLogic {
	return &ProjectUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectUpdate 更新项目（委托 portal-rpc）
func (l *ProjectUpdateLogic) ProjectUpdate(in *pb.UpdateOnecProjectReq) (*pb.UpdateOnecProjectResp, error) {
	if in.Id <= 0 {
		return nil, errorx.Msg("项目ID不能小于等于0")
	}
	if in.Name == "" {
		return nil, errorx.Msg("项目名称不能为空")
	}

	_, err := l.svcCtx.PortalProjectRpc.UpdateProject(l.ctx, &portalpb.PortalUpdateProjectReq{
		Id:          in.Id,
		Name:        in.Name,
		Description: in.Description,
		UpdatedBy:   in.UpdatedBy,
	})
	if err != nil {
		l.Errorf("调用 portal 更新项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新项目失败")
	}

	return &pb.UpdateOnecProjectResp{}, nil
}
