package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteClusterLogic {
	return &DeleteClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteClusterLogic) DeleteCluster(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先获取集群详情用于审计日志
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return "", err
	}

	_, err = l.svcCtx.ManagerRpc.ClusterDelete(l.ctx, &managerservice.DeleteClusterReq{
		Id:        req.Id,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("RPC调用删除集群失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  clusterResp.Uuid,
			Title:        "删除集群",
			ActionDetail: fmt.Sprintf("删除集群失败，集群名称：%s，环境：%s", clusterResp.Name, clusterResp.Environment),
			Status:       0,
		})

		return "", err
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterResp.Uuid,
		Title:        "删除集群",
		ActionDetail: fmt.Sprintf("成功删除集群，集群名称：%s，环境：%s，UUID：%s", clusterResp.Name, clusterResp.Environment, clusterResp.Uuid),
		Status:       1,
	})

	l.Infof("集群 [ID=%d] 删除成功", req.Id)
	return "集群删除成功", nil
}
