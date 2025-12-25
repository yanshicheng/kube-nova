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

type VPAGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 VPA YAML
func NewVPAGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VPAGetYamlLogic {
	return &VPAGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VPAGetYamlLogic) VPAGetYaml(req *types.VersionIdRequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	vpaOperator := client.VPA()

	var yamlStr string
	switch strings.ToLower(versionDetail.ResourceType) {
	case "deployment":
		deployment, err := client.Deployment().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 Deployment 详情失败: %v", err)
			return "", err
		}
		vpa, getErr := vpaOperator.GetByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: deployment.APIVersion,
			Kind:       deployment.Kind,
			Name:       deployment.Name,
		})
		if getErr != nil {
			l.Errorf("获取 VPA 失败: %v", getErr)
			return "", fmt.Errorf("未找到与资源 %s 关联的 VPA", versionDetail.ResourceName)
		}
		yamlStr, getErr = vpaOperator.GetYaml(vpa.Namespace, vpa.Name)
		if getErr != nil {
			l.Errorf("获取 VPA YAML 失败: %v", getErr)
			return "", getErr
		}
	case "statefulset":
		statefulSet, err := client.StatefulSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 详情失败: %v", err)
			return "", err
		}
		vpa, getErr := vpaOperator.GetByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: statefulSet.APIVersion,
			Kind:       statefulSet.Kind,
			Name:       statefulSet.Name,
		})
		if getErr != nil {
			l.Errorf("获取 VPA 失败: %v", getErr)
			return "", fmt.Errorf("未找到与资源 %s 关联的 VPA", versionDetail.ResourceName)
		}
		yamlStr, getErr = vpaOperator.GetYaml(vpa.Namespace, vpa.Name)
		if getErr != nil {
			l.Errorf("获取 VPA YAML 失败: %v", getErr)
			return "", getErr
		}
	case "daemonset":
		daemonSet, err := client.DaemonSet().Get(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 DaemonSet 详情失败: %v", err)
			return "", err
		}
		vpa, getErr := vpaOperator.GetByTargetRef(versionDetail.Namespace, k8stypes.TargetRefInfo{
			APIVersion: daemonSet.APIVersion,
			Kind:       daemonSet.Kind,
			Name:       daemonSet.Name,
		})
		if getErr != nil {
			l.Errorf("获取 VPA 失败: %v", getErr)
			return "", fmt.Errorf("未找到与资源 %s 关联的 VPA", versionDetail.ResourceName)
		}
		yamlStr, getErr = vpaOperator.GetYaml(vpa.Namespace, vpa.Name)
		if getErr != nil {
			l.Errorf("获取 VPA YAML 失败: %v", getErr)
			return "", getErr
		}
	default:
		l.Errorf("不支持的资源类型: %s", versionDetail.ResourceType)
		return "", fmt.Errorf("不支持的资源类型: %s", versionDetail.ResourceType)
	}

	return yamlStr, nil
}
