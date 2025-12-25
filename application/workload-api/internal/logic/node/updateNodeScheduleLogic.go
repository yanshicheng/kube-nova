package node

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateNodeScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateNodeScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateNodeScheduleLogic {
	return &UpdateNodeScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateNodeScheduleLogic) UpdateNodeSchedule(req *types.NodeDisableScheduleRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 根据状态启用或禁用调度
	nodeOperator := client.Node()

	// status: 1-启用调度, 2-禁用调度
	if req.Status == 1 {
		// 启用调度
		if err = nodeOperator.EnableScheduling(req.NodeName); err != nil {
			l.Errorf("启用节点调度失败: %v", err)
			// 记录失败审计日志
			l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
				ClusterUuid:  req.ClusterUuid,
				Title:        "节点启用调度",
				ActionDetail: fmt.Sprintf("用户 %s 启用节点 %s 调度失败, 错误: %v", username, req.NodeName, err),
				Status:       0,
			})
			return "", fmt.Errorf("启用节点调度失败")
		}
		l.Infof("成功启用节点调度: node=%s", req.NodeName)
		// 记录成功审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "节点启用调度",
			ActionDetail: fmt.Sprintf("用户 %s 启用节点 %s 调度成功, 节点现在可以接收新的 Pod 调度", username, req.NodeName),
			Status:       1,
		})
		return "启用调度成功", nil
	} else if req.Status == 2 {
		// 禁用调度
		if err = nodeOperator.DisableScheduling(req.NodeName); err != nil {
			l.Errorf("禁用节点调度失败: %v", err)
			// 记录失败审计日志
			l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
				ClusterUuid:  req.ClusterUuid,
				Title:        "节点禁用调度",
				ActionDetail: fmt.Sprintf("用户 %s 禁用节点 %s 调度失败, 错误: %v", username, req.NodeName, err),
				Status:       0,
			})
			return "", fmt.Errorf("禁用节点调度失败")
		}
		l.Infof("成功禁用节点调度: node=%s", req.NodeName)
		// 记录成功审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "节点禁用调度",
			ActionDetail: fmt.Sprintf("用户 %s 禁用节点 %s 调度成功, 节点将不再接收新的 Pod 调度", username, req.NodeName),
			Status:       1,
		})
		return "禁用调度成功", nil
	}

	return "", fmt.Errorf("无效的调度状态: %d", req.Status)
}
