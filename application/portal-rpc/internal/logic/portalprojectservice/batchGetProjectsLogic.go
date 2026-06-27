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

	var data []*pb.PortalProject
	for _, p := range projects {
		data = append(data, &pb.PortalProject{
			Id:          p.Id,
			Name:        p.Name,
			Uuid:        p.Uuid,
			IsSystem:    p.IsSystem,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			UpdatedBy:   p.UpdatedBy,
			CreatedAt:   p.CreatedAt.Unix(),
			UpdatedAt:   p.UpdatedAt.Unix(),
		})
	}

	return &pb.PortalListProjectsResp{
		Data:  data,
		Total: uint64(len(data)),
	}, nil
}
