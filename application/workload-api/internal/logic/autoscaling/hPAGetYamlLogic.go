package autoscaling

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HPAGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 HPA YAML
func NewHPAGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HPAGetYamlLogic {
	return &HPAGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HPAGetYamlLogic) HPAGetYaml(req *types.VersionIdRequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	hpaOperator := client.HPA()

	var yamlStr string
	var getErr error

	switch strings.ToLower(versionDetail.ResourceType) {
	case "deployment":
		deployment, err := client.Deployment().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 详情失败: %v", err)
			return "", err
		}
		detail, err := hpaOperator.GetByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: deployment.APIVersion,
			Kind:       deployment.Kind,
			Name:       deployment.Name,
		})
		if err != nil {
			l.Errorf("获取 HPA 详情失败: %v", err)
			return "", fmt.Errorf("未找到与资源 %s 关联的 HPA: %v", versionDetail.ResourceName, err)
		}
		yamlStr, getErr = hpaOperator.GetYaml(detail.Namespace, detail.Name)
		if getErr != nil {
			l.Errorf("获取 HPA YAML 失败: %v", getErr)
			return "", getErr
		}
	case "statefulset":
		statefulSet, err := client.StatefulSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 详情失败: %v", err)
			return "", err
		}
		detail, err := hpaOperator.GetByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: statefulSet.APIVersion,
			Kind:       statefulSet.Kind,
			Name:       statefulSet.Name,
		})
		if err != nil {
			l.Errorf("获取 HPA 详情失败: %v", err)
			return "", fmt.Errorf("未找到与资源 %s 关联的 HPA: %v", versionDetail.ResourceName, err)
		}
		yamlStr, getErr = hpaOperator.GetYaml(detail.Namespace, detail.Name)
		if getErr != nil {
			l.Errorf("获取 HPA YAML 失败: %v", getErr)
			return "", getErr
		}
	default:
		l.Errorf("不支持的资源类型: %s", versionDetail.ResourceType)
		return "", fmt.Errorf("不支持的资源类型: %s", versionDetail.ResourceType)
	}

	return yamlStr, nil
}
