package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewDeleteProjectClusterLogic 软删除项目集群配额，需确保没有工作空间在使用
func NewDeleteProjectClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectClusterLogic {
	return &DeleteProjectClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectClusterLogic) DeleteProjectCluster(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询项目集群配额信息用于审计日志
	clusterInfo, queryErr := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &pb.GetOnecProjectClusterByIdReq{
		Id: req.Id,
	})

	var clusterUuid string
	var projectId uint64
	if queryErr == nil && clusterInfo.Data != nil {
		clusterUuid = clusterInfo.Data.ClusterUuid
		projectId = uint64(clusterInfo.Data.ProjectId)
	}

	// 调用RPC服务删除项目集群配额
	_, err = l.svcCtx.ManagerRpc.ProjectClusterDel(l.ctx, &pb.DelOnecProjectClusterReq{
		Id: req.Id,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 删除项目集群配额, 配额ID: %d, 集群UUID: %s", username, req.Id, clusterUuid)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		ProjectId:    projectId,
		Title:        "删除项目集群配额",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("删除项目集群配额失败: %v", err)
		return "", fmt.Errorf("删除项目集群配额失败: %v", err)
	}

	return "项目集群配额删除成功", nil
}
