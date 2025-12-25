package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

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

// ProjectUpdate 更新项目信息
func (l *ProjectUpdateLogic) ProjectUpdate(in *pb.UpdateOnecProjectReq) (*pb.UpdateOnecProjectResp, error) {
	// 判断参数
	if in.Id <= 0 {
		l.Logger.Error("项目ID不能小于等于0")
		return nil, errorx.Msg("项目ID不能小于等于0")
	}
	if in.Name == "" {
		l.Logger.Error("项目名称不能为空")
		return nil, errorx.Msg("项目名称不能为空")
	}
	// 查询项目是否存在
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	// 更新项目信息
	project.Name = in.Name
	project.Description = in.Description
	project.UpdatedBy = in.UpdatedBy

	err = l.svcCtx.OnecProjectModel.Update(l.ctx, project)
	if err != nil {
		l.Logger.Errorf("更新项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新项目失败")
	}
	return &pb.UpdateOnecProjectResp{}, nil
}
