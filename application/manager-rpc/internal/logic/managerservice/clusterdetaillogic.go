package managerservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterDetailLogic {
	return &ClusterDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取集群详情
func (l *ClusterDetailLogic) ClusterDetail(in *pb.ClusterDetailReq) (*pb.ClusterDetailResp, error) {

	// 1. 查询集群信息
	cluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群不存在 [id=%d]", in.Id)
			return nil, errorx.Msg("集群不存在")
		}
		l.Errorf("查询集群失败: %v", err)
		return nil, errorx.Msg("查询集群失败")
	}

	url, err := l.svcCtx.Storage.GetStorageUrl(l.ctx, &storageservice.GetStorageUrlRequest{})
	if err != nil {
		l.Errorf("获取存储的访问 url 失败: %v", err)
		return nil, errorx.Msg("获取存储的访问 url 失败")
	}
	// 2. 构建响应
	resp := &pb.ClusterDetailResp{
		Id:               cluster.Id,
		Name:             cluster.Name,
		Avatar:           fmt.Sprintf("%s/%s", url.Url, cluster.Avatar),
		Uuid:             cluster.Uuid,
		Description:      cluster.Description,
		ClusterType:      cluster.ClusterType,
		Environment:      cluster.Environment,
		Region:           cluster.Region,
		Zone:             cluster.Zone,
		Datacenter:       cluster.Datacenter,
		Provider:         cluster.Provider,
		IsManaged:        cluster.IsManaged,
		NodeLb:           cluster.NodeLb,
		MasterLb:         cluster.MasterLb,
		IngressDomain:    cluster.IngressDomain,
		Status:           cluster.Status,
		HealthStatus:     cluster.HealthStatus,
		LastSyncAt:       cluster.LastSyncAt.Unix(),
		Version:          cluster.Version,
		GitCommit:        cluster.GitCommit,
		Platform:         cluster.Platform,
		VersionBuildAt:   cluster.VersionBuildAt.Unix(),
		ClusterCreatedAt: cluster.ClusterCreatedAt.Unix(),
		CostCenter:       cluster.CostCenter,
		BusinessUnit:     cluster.BusinessUnit,
		OwnerTeam:        cluster.OwnerTeam,
		OwnerEmail:       cluster.OwnerEmail,
		Priority:         cluster.Priority,
		CreatedBy:        cluster.CreatedBy,
		UpdatedBy:        cluster.UpdatedBy,
		CreatedAt:        cluster.CreatedAt.Unix(),
		UpdatedAt:        cluster.UpdatedAt.Unix(),
	}
	return resp, nil
}
