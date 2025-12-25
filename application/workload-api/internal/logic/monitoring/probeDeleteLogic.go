package monitoring

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProbeDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Probe
func NewProbeDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProbeDeleteLogic {
	return &ProbeDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProbeDeleteLogic) ProbeDelete(req *types.MonitoringResourceNameRequest) (resp string, err error) {

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 Probe 操作器
	probeOp := client.Probe()

	// 调用 Delete 方法
	deleteErr := probeOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 Probe 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 Probe",
			ActionDetail: fmt.Sprintf("删除 Probe 失败: %s/%s, 错误: %v", req.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 Probe 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 Probe",
		ActionDetail: fmt.Sprintf("删除 Probe: %s/%s", req.Namespace, req.Name),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	return fmt.Sprintf("Probe %s/%s 删除成功", req.Namespace, req.Name), nil
}
