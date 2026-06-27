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
	platformId, _ := l.ctx.Value("platformId").(uint64)
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

	if !isSuperAdmin(currentRoles(l.ctx)) {
		userId := currentUserID(l.ctx)
		if userId == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
		if platformId == 0 {
			platformId = currentPlatformID(l.ctx)
		}
		if platformId == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
		allowedResp, err := l.svcCtx.ProjectRpc.ListProjects(l.ctx, &pb.PortalListProjectsReq{
			Page:       1,
			PageSize:   uint64(len(ids)),
			IsSystem:   0,
			UserId:     userId,
			PlatformId: platformId,
		})
		if err != nil {
			l.Errorf("校验项目权限失败: %v", err)
			return nil, err
		}
		allowed := make(map[uint64]struct{}, len(allowedResp.Data))
		for _, item := range allowedResp.Data {
			allowed[item.Id] = struct{}{}
		}
		filtered := ids[:0]
		for _, id := range ids {
			if _, ok := allowed[id]; ok {
				filtered = append(filtered, id)
			}
		}
		ids = filtered
		if len(ids) == 0 {
			return &types.PortalSearchProjectResp{Data: []types.PortalProject{}, Total: 0}, nil
		}
	}

	rpcResp, err := l.svcCtx.ProjectRpc.BatchGetProjects(l.ctx, &pb.PortalBatchGetProjectsReq{
		Ids: ids,
	})
	if err != nil {
		l.Errorf("批量获取项目失败: %v", err)
		return nil, err
	}

	return &types.PortalSearchProjectResp{
		Data:  projectsToType(rpcResp.Data),
		Total: rpcResp.Total,
	}, nil
}
