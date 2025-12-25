package canary

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Canary YAML
func NewCanaryGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryGetYamlLogic {
	return &CanaryGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryGetYamlLogic) CanaryGetYaml(req *types.CanaryNameRequest) (resp string, err error) {
	workload, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{
		Id: req.WorkloadId,
	})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败: %v", err)
	}
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workload.Data.ClusterUuid)

	canaryOperator := client.Flagger()
	yamlStr, err := canaryOperator.GetYaml(workload.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Canary YAML 失败: %v", err)
		return "", fmt.Errorf("获取 Canary YAML 失败: %v", err)
	}

	return yamlStr, nil
}
