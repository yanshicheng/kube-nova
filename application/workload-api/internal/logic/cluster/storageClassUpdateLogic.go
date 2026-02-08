package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type StorageClassUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 StorageClass
func NewStorageClassUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StorageClassUpdateLogic {
	return &StorageClassUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StorageClassUpdateLogic) StorageClassUpdate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	scOp := client.StorageClasses()

	var sc storagev1.StorageClass
	if err := yaml.Unmarshal([]byte(req.YamlStr), &sc); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 获取旧的 StorageClass 用于对比
	oldSC, err := scOp.Get(sc.Name)
	if err != nil {
		l.Errorf("获取原 StorageClass 失败: %v", err)
		return "", fmt.Errorf("获取原 StorageClass 失败")
	}

	// 注入注解
	utils.AddAnnotations(&sc.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: sc.Name,
	})

	// 对比参数变更
	paramDiff := CompareStringMaps(oldSC.Parameters, sc.Parameters)
	paramDiffDetail := ""
	if HasMapChanges(paramDiff) {
		paramDiffDetail = "参数变更: " + BuildMapDiffDetail(paramDiff, true)
	}

	// 对比标签变更
	labelDiff := CompareStringMaps(oldSC.Labels, sc.Labels)
	labelDiffDetail := ""
	if HasMapChanges(labelDiff) {
		labelDiffDetail = "标签变更: " + BuildMapDiffDetail(labelDiff, true)
	}

	// 对比挂载选项变更
	mountAdded, mountDeleted := CompareStringSlices(oldSC.MountOptions, sc.MountOptions)
	mountDiffDetail := BuildSliceDiffDetail("挂载选项", mountAdded, mountDeleted)

	// 检查 AllowVolumeExpansion 变更
	var expansionChange string
	oldExpansion := oldSC.AllowVolumeExpansion != nil && *oldSC.AllowVolumeExpansion
	newExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
	if oldExpansion != newExpansion {
		if newExpansion {
			expansionChange = "允许扩展: 否 → 是"
		} else {
			expansionChange = "允许扩展: 是 → 否"
		}
	}

	_, err = scOp.Update(&sc)
	if err != nil {
		l.Errorf("更新 StorageClass 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "更新 StorageClass",
			ActionDetail: fmt.Sprintf("用户 %s 更新 StorageClass %s 失败, 错误: %v", username, sc.Name, err),
			Status:       0,
		})
		return "", fmt.Errorf("更新 StorageClass 失败")
	}

	l.Infof("用户: %s, 成功更新 StorageClass: %s", username, sc.Name)

	// 构建详细审计信息
	var changeParts []string
	changeParts = append(changeParts, fmt.Sprintf("用户 %s 成功更新 StorageClass %s", username, sc.Name))
	changeParts = append(changeParts, fmt.Sprintf("Provisioner: %s", sc.Provisioner))

	if paramDiffDetail != "" {
		changeParts = append(changeParts, paramDiffDetail)
	}
	if labelDiffDetail != "" {
		changeParts = append(changeParts, labelDiffDetail)
	}
	if mountDiffDetail != "" {
		changeParts = append(changeParts, mountDiffDetail)
	}
	if expansionChange != "" {
		changeParts = append(changeParts, expansionChange)
	}

	auditDetail := joinNonEmpty(", ", changeParts...)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "更新 StorageClass",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "更新 StorageClass 成功", nil
}
