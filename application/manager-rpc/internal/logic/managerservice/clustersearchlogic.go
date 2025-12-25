package managerservicelogic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterSearchLogic {
	return &ClusterSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterSearchLogic) ClusterSearch(in *pb.SearchClusterReq) (*pb.SearchClusterResp, error) {
	// 1. 设置默认参数
	l.setDefaultParams(in)

	// 2. 构建查询条件
	query, args := l.buildQueryConditions(in)

	// 3. 获取存储 URL
	storageUrl, err := l.getStorageUrl()
	if err != nil {
		return nil, err
	}

	// 4. 执行数据库查询
	clusters, total, err := l.svcCtx.OnecClusterModel.Search(
		l.ctx,
		in.OrderField,
		in.IsAsc,
		in.Page,
		in.PageSize,
		query,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到匹配的集群")
			return &pb.SearchClusterResp{Total: 0, Data: nil}, nil
		}
		l.Errorf("搜索集群失败: %v", err)
		return nil, errorx.Msg("搜索集群失败")
	}

	// 5. 批量获取集群资源信息并构建响应
	return l.buildResponse(clusters, total, storageUrl), nil
}

// setDefaultParams 设置默认参数
func (l *ClusterSearchLogic) setDefaultParams(in *pb.SearchClusterReq) {
	if in.Page <= 0 {
		in.Page = vars.Page
	}
	if in.PageSize <= 0 {
		in.PageSize = vars.PageSize
	}
	if in.OrderField == "" {
		in.OrderField = "created_at"
	}
}

// buildQueryConditions 构建查询条件
func (l *ClusterSearchLogic) buildQueryConditions(in *pb.SearchClusterReq) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if name := strings.TrimSpace(in.Name); name != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+name+"%")
	}

	if env := strings.TrimSpace(in.Environment); env != "" {
		conditions = append(conditions, "`environment` LIKE ?")
		args = append(args, "%"+env+"%")
	}

	if uuid := strings.TrimSpace(in.Uuid); uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, uuid)
	}

	return strings.Join(conditions, " AND "), args
}

// getStorageUrl 获取存储访问 URL
func (l *ClusterSearchLogic) getStorageUrl() (string, error) {
	resp, err := l.svcCtx.Storage.GetStorageUrl(l.ctx, &storageservice.GetStorageUrlRequest{})
	if err != nil {
		l.Errorf("获取存储的访问 URL 失败: %v", err)
		return "", errorx.Msg("获取存储的访问 URL 失败")
	}
	return resp.Url, nil
}

// buildResponse 构建响应数据
func (l *ClusterSearchLogic) buildResponse(clusters []*model.OnecCluster, total uint64, storageUrl string) *pb.SearchClusterResp {
	resp := &pb.SearchClusterResp{
		Total: total,
		Data:  make([]*pb.Cluster, 0, len(clusters)),
	}

	for _, cluster := range clusters {
		// 获取集群资源信息
		resource, err := l.svcCtx.OnecClusterResourceModel.FindOneByClusterUuid(l.ctx, cluster.Uuid)
		if err != nil {
			if !errors.Is(err, model.ErrNotFound) {
				l.Errorf("获取集群资源信息失败 [uuid=%s]: %v", cluster.Uuid, err)
			}
			// 资源信息获取失败时，使用空资源
			resource = &model.OnecClusterResource{}
		}

		resp.Data = append(resp.Data, l.convertToProto(cluster, resource, storageUrl))
	}

	return resp
}

// convertToProto 将数据库模型转换为 Proto 对象
func (l *ClusterSearchLogic) convertToProto(cluster *model.OnecCluster, resource *model.OnecClusterResource, storageUrl string) *pb.Cluster {
	// 处理头像 URL
	avatar := cluster.Avatar
	if avatar != "" && storageUrl != "" {
		avatar = fmt.Sprintf("%s%s", storageUrl, cluster.Avatar)
	}

	return &pb.Cluster{
		Id:           cluster.Id,
		Name:         cluster.Name,
		Avatar:       avatar,
		Uuid:         cluster.Uuid,
		Environment:  cluster.Environment,
		ClusterType:  cluster.ClusterType,
		CpuUsage:     l.calculateUsagePercent(resource.CpuAllocatedTotal, resource.CpuPhysicalCapacity),
		MemoryUsage:  l.calculateUsagePercent(resource.MemAllocatedTotal, resource.MemPhysicalCapacity),
		PodUsage:     l.calculateIntUsagePercent(resource.PodsAllocatedTotal, resource.PodsPhysicalCapacity),
		StorageUsage: l.calculateUsagePercent(resource.StorageAllocatedTotal, resource.StoragePhysicalCapacity),
		Version:      cluster.Version,
		Status:       cluster.Status,
		HealthStatus: cluster.HealthStatus,
		CreatedAt:    cluster.CreatedAt.Unix(),
	}
}

// calculateUsagePercent 计算资源使用率百分比（float64 类型）
func (l *ClusterSearchLogic) calculateUsagePercent(used, total float64) string {
	if total <= 0 {
		return "0.0%"
	}
	percent := (used / total) * 100
	return fmt.Sprintf("%.1f%%", percent)
}

// calculateIntUsagePercent 计算资源使用率百分比（int64 类型）
func (l *ClusterSearchLogic) calculateIntUsagePercent(used, total int64) string {
	if total <= 0 {
		return "0.0%"
	}
	percent := (float64(used) / float64(total)) * 100
	return fmt.Sprintf("%.1f%%", percent)
}
