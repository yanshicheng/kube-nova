package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMemberDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMemberDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMemberDeleteLogic {
	return &ProjectMemberDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMemberDeleteLogic) ProjectMemberDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	if err := l.svcCtx.ProjectMemberModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目成员删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
