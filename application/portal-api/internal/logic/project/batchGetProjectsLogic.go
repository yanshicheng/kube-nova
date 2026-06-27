package project

import (
	"context"
	"strconv"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchGetProjectsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchGetProjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetProjectsLogic {
	return &BatchGetProjectsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchGetProjectsLogic) BatchGetProjects(req *types.PortalBatchGetProjectReq) (resp *types.PortalSearchProjectResp, err error) {
	idStrs := strings.Split(req.Ids, ",")
	ids := make([]uint64, 0, len(idStrs))
	for _, s := range idStrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
	}

	rpcResp, err := l.svcCtx.ProjectRpc.BatchGetProjects(l.ctx, &pb.PortalBatchGetProjectsReq{
		Ids: ids,
	})
	if err != nil {
		l.Errorf("批量获取项目失败: %v", err)
		return nil, err
	}

	items := make([]types.PortalProject, 0, len(rpcResp.Data))
	for _, p := range rpcResp.Data {
		items = append(items, types.PortalProject{
			Id:          p.Id,
			Name:        p.Name,
			Uuid:        p.Uuid,
			IsSystem:    p.IsSystem,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			UpdatedBy:   p.UpdatedBy,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	return &types.PortalSearchProjectResp{
		Data:  items,
		Total: rpcResp.Total,
	}, nil
}
