package portalprojectservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchGetProjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchGetProjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetProjectsLogic {
	return &BatchGetProjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchGetProjectsLogic) BatchGetProjects(in *pb.PortalBatchGetProjectsReq) (*pb.PortalListProjectsResp, error) {
	if len(in.Ids) == 0 {
		return &pb.PortalListProjectsResp{Data: []*pb.PortalProject{}, Total: 0}, nil
	}

	projects, err := l.svcCtx.OnecProjectModel.BatchFindByIds(l.ctx, in.Ids)
	if err != nil {
		l.Errorf("批量查询项目失败: %v", err)
		return nil, fmt.Errorf("批量查询项目失败: %v", err)
	}

	data := projectsToPb(projects)

	return &pb.PortalListProjectsResp{
		Data:  data,
		Total: uint64(len(data)),
	}, nil
}
