package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectGetLogic {
	return &ProjectGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectGetLogic) ProjectGet(in *pb.GetByIdReq) (*pb.GetProjectResp, error) {
	data, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetProjectResp{Data: projectToPb(data)}, nil
}
