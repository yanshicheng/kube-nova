package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/utils"

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

// -----------------------项目表，记录项目信息-----------------------
func (l *ProjectAddLogic) ProjectAdd(in *pb.AddOnecProjectReq) (*pb.AddOnecProjectResp, error) {
	// 校验参数
	if in.Name == "" || in.CreatedBy == "" || in.UpdatedBy == "" {
		l.Errorf("参数错误: %v", in)
		return nil, errorx.Msg("参数错误")
	}
	_, err := l.svcCtx.OnecProjectModel.Insert(l.ctx, &model.OnecProject{
		Name:        in.Name,
		Description: in.Description,
		Uuid:        utils.NewUuid(),
		IsSystem:    in.IsSystem,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.UpdatedBy,
	})
	if err != nil {
		l.Errorf("添加项目失败: %v", err)
		return nil, errorx.Msg("添加项目失败")
	}
	return &pb.AddOnecProjectResp{}, nil
}
