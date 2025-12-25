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

type UpdateClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateClusterLogic {
	return &UpdateClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateClusterLogic) UpdateCluster(req *types.UpdateClusterRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	l.Infof("用户 [%s] 开始更新集群 [ID=%d]", username, req.Id)

	// 先获取集群详情用于审计日志
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return "", fmt.Errorf("获取集群详情失败: %s", err.Error())
	}

	// 构建RPC请求
	rpcReq := &pb.UpdateClusterReq{
		Id:            req.Id,
		Name:          req.Name,
		Description:   req.Description,
		ClusterType:   req.ClusterType,
		Environment:   req.Environment,
		Region:        req.Region,
		Zone:          req.Zone,
		Datacenter:    req.Datacenter,
		Provider:      req.Provider,
		IsManaged:     req.IsManaged,
		NodeLb:        req.NodeLb,
		MasterLb:      req.MasterLb,
		IngressDomain: req.IngressDomain,
		UpdatedBy:     username,
	}

	// 调用RPC更新集群
	_, err = l.svcCtx.ManagerRpc.ClusterUpdate(l.ctx, rpcReq)
	if err != nil {
		l.Errorf("RPC调用更新集群失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  clusterResp.Uuid,
			Title:        "更新集群配置",
			ActionDetail: fmt.Sprintf("更新集群配置失败，集群名称：%s，环境：%s", req.Name, req.Environment),
			Status:       0,
		})

		return "", fmt.Errorf("更新集群失败: %s", err.Error())
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterResp.Uuid,
		Title:        "更新集群配置",
		ActionDetail: fmt.Sprintf("成功更新集群配置，集群名称：%s -> %s，环境：%s -> %s，类型：%s -> %s，地域：%s -> %s", clusterResp.Name, req.Name, clusterResp.Environment, req.Environment, clusterResp.ClusterType, req.ClusterType, clusterResp.Region, req.Region),
		Status:       1,
	})

	l.Infof("集群 [ID=%d] 更新成功", req.Id)
	return "集群更新成功", nil
}
