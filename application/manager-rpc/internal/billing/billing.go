package billing

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
)

// Service 账单服务接口
type Service interface {
	// GenerateAll 立即生成全部账单
	GenerateAll(ctx context.Context, option *GenerateOption) (*GenerateResult, error)

	// GenerateByCluster 生成某个集群的所有账单
	GenerateByCluster(ctx context.Context, clusterUuid string, option *GenerateOption) (*GenerateResult, error)

	// GenerateByProject 生成某个项目的所有账单
	GenerateByProject(ctx context.Context, projectId uint64, option *GenerateOption) (*GenerateResult, error)

	// GenerateByPriceConfig 生成关联到某个费用配置的所有账单
	GenerateByPriceConfig(ctx context.Context, priceConfigId uint64, option *GenerateOption) (*GenerateResult, error)

	// GenerateByProjectCluster 生成单个项目集群的账单（通过项目集群ID）
	GenerateByProjectCluster(ctx context.Context, projectClusterId uint64, option *GenerateOption) error

	// GenerateByClusterAndProject 生成单个项目集群的账单（通过集群UUID和项目ID）
	GenerateByClusterAndProject(ctx context.Context, clusterUuid string, projectId uint64, option *GenerateOption) error
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	ClusterModel              model.OnecClusterModel
	ProjectModel              model.OnecProjectModel
	ProjectClusterModel       model.OnecProjectClusterModel
	ProjectWorkspaceModel     model.OnecProjectWorkspaceModel
	ProjectApplicationModel   model.OnecProjectApplicationModel
	BillingPriceConfigModel   model.OnecBillingPriceConfigModel
	BillingConfigBindingModel model.OnecBillingConfigBindingModel
	BillingStatementModel     model.OnecBillingStatementModel
}

// billingService 账单服务实现
type billingService struct {
	config ServiceConfig
}

// NewService 创建账单服务实例
func NewService(config ServiceConfig) Service {
	return &billingService{
		config: config,
	}
}

// createGenerator 创建账单生成器
func (s *billingService) createGenerator(ctx context.Context) *Generator {
	return NewGenerator(ctx, GeneratorConfig{
		ClusterModel:              s.config.ClusterModel,
		ProjectModel:              s.config.ProjectModel,
		ProjectClusterModel:       s.config.ProjectClusterModel,
		ProjectWorkspaceModel:     s.config.ProjectWorkspaceModel,
		ProjectApplicationModel:   s.config.ProjectApplicationModel,
		BillingPriceConfigModel:   s.config.BillingPriceConfigModel,
		BillingConfigBindingModel: s.config.BillingConfigBindingModel,
		BillingStatementModel:     s.config.BillingStatementModel,
	})
}

// GenerateAll 立即生成全部账单
func (s *billingService) GenerateAll(ctx context.Context, option *GenerateOption) (*GenerateResult, error) {
	logger := logx.WithContext(ctx)
	logger.Info("账单服务: 开始生成全部账单")

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeDaily,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	result, err := generator.GenerateAll(option)
	if err != nil {
		logger.Errorf("账单服务: 生成全部账单失败, err: %v", err)
		return nil, err
	}

	logger.Infof("账单服务: 生成全部账单完成, total: %d, success: %d, failed: %d",
		result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByCluster 生成某个集群的所有账单
func (s *billingService) GenerateByCluster(ctx context.Context, clusterUuid string, option *GenerateOption) (*GenerateResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("账单服务: 开始生成集群账单, clusterUuid: %s", clusterUuid)

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeDaily,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	result, err := generator.GenerateByCluster(clusterUuid, option)
	if err != nil {
		logger.Errorf("账单服务: 生成集群账单失败, clusterUuid: %s, err: %v", clusterUuid, err)
		return nil, err
	}

	logger.Infof("账单服务: 生成集群账单完成, clusterUuid: %s, total: %d, success: %d, failed: %d",
		clusterUuid, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByProject 生成某个项目的所有账单
func (s *billingService) GenerateByProject(ctx context.Context, projectId uint64, option *GenerateOption) (*GenerateResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("账单服务: 开始生成项目账单, projectId: %d", projectId)

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeDaily,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	result, err := generator.GenerateByProject(projectId, option)
	if err != nil {
		logger.Errorf("账单服务: 生成项目账单失败, projectId: %d, err: %v", projectId, err)
		return nil, err
	}

	logger.Infof("账单服务: 生成项目账单完成, projectId: %d, total: %d, success: %d, failed: %d",
		projectId, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByPriceConfig 生成关联到某个费用配置的所有账单
func (s *billingService) GenerateByPriceConfig(ctx context.Context, priceConfigId uint64, option *GenerateOption) (*GenerateResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("账单服务: 开始生成价格配置关联账单, priceConfigId: %d", priceConfigId)

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeConfigChange,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	result, err := generator.GenerateByPriceConfig(priceConfigId, option)
	if err != nil {
		logger.Errorf("账单服务: 生成价格配置关联账单失败, priceConfigId: %d, err: %v", priceConfigId, err)
		return nil, err
	}

	logger.Infof("账单服务: 生成价格配置关联账单完成, priceConfigId: %d, total: %d, success: %d, failed: %d",
		priceConfigId, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByProjectCluster 生成单个项目集群的账单（通过项目集群ID）
func (s *billingService) GenerateByProjectCluster(ctx context.Context, projectClusterId uint64, option *GenerateOption) error {
	logger := logx.WithContext(ctx)
	logger.Infof("账单服务: 开始生成单个项目集群账单, projectClusterId: %d", projectClusterId)

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeDaily,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	err := generator.GenerateByProjectCluster(projectClusterId, option)
	if err != nil {
		logger.Errorf("账单服务: 生成单个项目集群账单失败, projectClusterId: %d, err: %v", projectClusterId, err)
		return err
	}

	logger.Infof("账单服务: 生成单个项目集群账单完成, projectClusterId: %d", projectClusterId)

	return nil
}

// GenerateByClusterAndProject 生成单个项目集群的账单（通过集群UUID和项目ID）
func (s *billingService) GenerateByClusterAndProject(ctx context.Context, clusterUuid string, projectId uint64, option *GenerateOption) error {
	logger := logx.WithContext(ctx)
	logger.Infof("账单服务: 开始生成项目集群账单, clusterUuid: %s, projectId: %d", clusterUuid, projectId)

	if option == nil {
		option = &GenerateOption{
			StatementType: StatementTypeDaily,
			CreatedBy:     "system",
		}
	}

	generator := s.createGenerator(ctx)
	err := generator.GenerateByClusterAndProject(clusterUuid, projectId, option)
	if err != nil {
		logger.Errorf("账单服务: 生成项目集群账单失败, clusterUuid: %s, projectId: %d, err: %v", clusterUuid, projectId, err)
		return err
	}

	logger.Infof("账单服务: 生成项目集群账单完成, clusterUuid: %s, projectId: %d", clusterUuid, projectId)

	return nil
}
