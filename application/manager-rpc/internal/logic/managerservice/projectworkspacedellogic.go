package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectWorkspaceDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceDelLogic {
	return &ProjectWorkspaceDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceDel 删除项目工作空间
func (l *ProjectWorkspaceDelLogic) ProjectWorkspaceDel(in *pb.DelOnecProjectWorkspaceReq) (*pb.DelOnecProjectWorkspaceResp, error) {
	// 查询工作空间是否存在
	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}
	clusterBind, err := l.svcCtx.OnecProjectClusterModel.FindOneByClusterUuidProjectId(l.ctx, workspace.ClusterUuid, workspace.ProjectClusterId)
	// 查询是否还有 deployment 资源
	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workspace.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败，ID: %s, 错误: %v", workspace.ClusterUuid, err)
		return nil, errorx.Msg("获取集群客户端失败")
	}
	deploymentList, err := clusterClient.Deployment().List(workspace.Namespace, types.ListRequest{
		Search:   "",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		l.Errorf("查询 deployment 失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("查询 deployment 失败")
	}
	if deploymentList.Total > 0 {
		l.Errorf("工作空间下存在 deployment 资源，请先删除 deployment 资源，命名空间: %s", workspace.Namespace)
		return nil, errorx.Msg("工作空间下存在 deployment 资源，请先删除 deployment 资源")
	}
	// 判断是否存在 sts 资源
	stsList, err := clusterClient.StatefulSet().List(workspace.Namespace, types.ListRequest{
		Search:   "",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		l.Errorf("查询 sts 列表失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("查询 sts 列表失败")
	}
	if stsList.Total > 0 {
		l.Errorf("工作空间下存在 sts 资源，请先删除 sts 资源，命名空间: %s", workspace.Namespace)
	}
	// 判断是否存在 cronjob 资源
	cronjobList, err := clusterClient.CronJob().List(workspace.Namespace, types.ListRequest{
		Search:   "",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		l.Errorf("查询 cronjob 列表失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("查询 cronjob 列表失败")
	}
	if cronjobList.Total > 0 {
		l.Errorf("工作空间下存在 cronjob 资源，请先删除 cronjob 资源，命名空间: %s", workspace.Namespace)
	}
	// 判断是否存在 daemonset 资源
	daemonsetList, err := clusterClient.DaemonSet().List(workspace.Namespace, types.ListRequest{
		Search:   "",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		l.Errorf("查询 daemonset 列表失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("查询 daemonset 列表失败")
	}
	if daemonsetList.Total > 0 {
		l.Errorf("工作空间下存在 daemonset 资源，请先删除 daemonset 资源，命名空间: %s", workspace.Namespace)
		return nil, errorx.Msg("工作空间下存在 daemonset 资源，请先删除 daemonset 资源")
	}
	// 删除 namespace
	err = clusterClient.Namespaces().Delete(workspace.Namespace)
	if err != nil {
		l.Errorf("删除 namespace 失败，命名空间: %s, 错误: %v", workspace.Namespace, err)
		return nil, errorx.Msg("删除 namespace 失败")
	}
	l.Infof("删除 namespace 成功，命名空间: %s", workspace.Namespace)
	// 执行软删除
	err = l.svcCtx.OnecProjectWorkspaceModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除工作空间失败")
	}
	l.Infof("删除工作空间成功，ID: %d, 命名空间: %s", in.Id, workspace.Namespace)
	// 重新更新项目资源
	err = l.svcCtx.OnecProjectModel.SyncProjectClusterResourceAllocation(l.ctx, clusterBind.Id)
	if err != nil {
		l.Errorf("更新项目集群资源失败，项目集群ID: %d, 错误: %v", clusterBind.Id, err)
		return nil, errorx.Msg("更新项目集群资源失败: " + err.Error())
	}
	l.Infof("更新项目集群资源成功，项目集群ID: %d", clusterBind.Id)
	return &pb.DelOnecProjectWorkspaceResp{}, nil
}
