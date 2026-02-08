package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 PV
func NewPVCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCreateLogic {
	return &PVCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCreateLogic) PVCreate(req *types.ClusterResourceYamlRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	pvOp := client.PersistentVolumes()

	var pv corev1.PersistentVolume
	if err := yaml.Unmarshal([]byte(req.YamlStr), &pv); err != nil {
		l.Errorf("解析 YAML 失败: %v", err)
		return "", fmt.Errorf("解析 YAML 失败: %v", err)
	}

	// 提取容量信息
	capacity := ""
	if qty, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = qty.String()
	}

	// 注入注解
	utils.AddAnnotations(&pv.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName: pv.Name,
	})

	// 提取访问模式
	var accessModes []string
	for _, mode := range pv.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	// 提取回收策略
	reclaimPolicy := "Delete"
	if pv.Spec.PersistentVolumeReclaimPolicy != "" {
		reclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)
	}

	// 提取存储源类型
	sourceType := getPVSourceType(&pv)

	configDetail := FormatPVConfig(capacity, pv.Spec.StorageClassName, reclaimPolicy, accessModes)

	_, err = pvOp.Create(&pv)
	if err != nil {
		l.Errorf("创建 PV 失败: %v", err)
		// 记录失败审计日志
		l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "创建 PersistentVolume",
			ActionDetail: fmt.Sprintf("用户 %s 创建 PersistentVolume %s 失败, 容量: %s, 错误: %v", username, pv.Name, capacity, err),
			Status:       0,
		})
		return "", fmt.Errorf("创建 PV 失败")
	}

	l.Infof("用户: %s, 成功创建 PV: %s", username, pv.Name)

	// 构建详细审计信息
	auditDetail := fmt.Sprintf("用户 %s 成功创建 PersistentVolume %s, %s, 存储源类型: %s",
		username, pv.Name, configDetail, sourceType)

	// 记录成功审计日志
	l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "创建 PersistentVolume",
		ActionDetail: auditDetail,
		Status:       1,
	})
	return "创建 PV 成功", nil
}

// getPVSourceType 获取 PV 存储源类型
func getPVSourceType(pv *corev1.PersistentVolume) string {
	source := pv.Spec.PersistentVolumeSource
	switch {
	case source.NFS != nil:
		return fmt.Sprintf("NFS (Server: %s, Path: %s)", source.NFS.Server, source.NFS.Path)
	case source.HostPath != nil:
		return fmt.Sprintf("HostPath (Path: %s)", source.HostPath.Path)
	case source.CSI != nil:
		return fmt.Sprintf("CSI (Driver: %s)", source.CSI.Driver)
	case source.Local != nil:
		return fmt.Sprintf("Local (Path: %s)", source.Local.Path)
	case source.ISCSI != nil:
		return fmt.Sprintf("ISCSI (Portal: %s)", source.ISCSI.TargetPortal)
	case source.FC != nil:
		return fmt.Sprintf("FC (WWNs: %s)", strings.Join(source.FC.TargetWWNs, ","))
	case source.RBD != nil:
		return fmt.Sprintf("RBD (Image: %s)", source.RBD.RBDImage)
	default:
		return "Unknown"
	}
}
