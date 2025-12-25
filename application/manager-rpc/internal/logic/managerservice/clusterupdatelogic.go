package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	utils2 "github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterUpdateLogic {
	return &ClusterUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改集群
func (l *ClusterUpdateLogic) ClusterUpdate(in *pb.UpdateClusterReq) (*pb.UpdateClusterResp, error) {
	// 获取旧集群
	oldCluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [id=%d]", in.Id)
			return nil, errorx.Msg("集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群失败")
	}

	if in.Name != "" && in.Name != oldCluster.Name {
		oldCluster.Name = in.Name
	}
	if in.Description != "" && in.Description != oldCluster.Description {
		oldCluster.Description = in.Description
	}

	if in.Environment != "" && in.Environment != oldCluster.Environment {
		oldCluster.Environment = in.Environment
	}

	if in.Region != "" && in.Region != oldCluster.Region {
		oldCluster.Region = in.Region
	}

	if in.Zone != "" && in.Zone != oldCluster.Zone {
		oldCluster.Zone = in.Zone
	}

	if in.Datacenter != "" && in.Datacenter != oldCluster.Datacenter {
		oldCluster.Datacenter = in.Datacenter
	}

	if in.Provider != "" && in.Provider != oldCluster.Provider {
		oldCluster.Provider = in.Provider
	}

	if in.IsManaged != oldCluster.IsManaged {
		oldCluster.IsManaged = in.IsManaged
	}
	if in.NodeLb != "" {
		nodeLb, err := utils2.ValidateAndCleanAddresses(in.NodeLb)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
		if nodeLb != "" && nodeLb != oldCluster.NodeLb {
			oldCluster.NodeLb = nodeLb
		}
	}

	if in.MasterLb != "" {
		masterLb, err := utils2.ValidateAndCleanAddresses(in.MasterLb)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
		if masterLb != "" && masterLb != oldCluster.MasterLb {
			oldCluster.MasterLb = masterLb
		}
	}

	if in.IngressDomain != "" {
		ingressDomain, err := utils2.ValidateAndCleanDomainSuffixes(in.IngressDomain)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
		if ingressDomain != "" && ingressDomain != oldCluster.IngressDomain {
			oldCluster.IngressDomain = ingressDomain
		}
	}

	// 更新修改人和修改时间
	if in.UpdatedBy != "" {
		oldCluster.UpdatedBy = in.UpdatedBy
	}
	err = l.svcCtx.OnecClusterModel.Update(l.ctx, oldCluster)
	if err != nil {
		l.Errorf("更新集群失败: %v", err)
		return nil, errorx.Msg("更新集群失败")
	}
	// 如果集群信息有变更，更新数据库

	l.Infof("集群更新成功 [id=%d, name=%s, uuid=%s]", in.Id, oldCluster.Name, oldCluster.Uuid)

	return &pb.UpdateClusterResp{}, nil

}

// IsInList 辅助函数：检查字符串是否在列表中
func IsInList(target string, list []string) bool {
	for _, item := range list {
		if target == item {
			return true
		}
	}
	return false
}
