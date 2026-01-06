package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

// 不计费配置ID（系统内置，id=1 表示不计费）
const NoBillingPriceConfigId = 1

// Generator 账单生成器
type Generator struct {
	ctx                       context.Context
	logger                    logx.Logger
	calculator                *Calculator
	clusterModel              model.OnecClusterModel
	projectModel              model.OnecProjectModel
	projectClusterModel       model.OnecProjectClusterModel
	projectWorkspaceModel     model.OnecProjectWorkspaceModel
	projectApplicationModel   model.OnecProjectApplicationModel
	billingPriceConfigModel   model.OnecBillingPriceConfigModel
	billingConfigBindingModel model.OnecBillingConfigBindingModel
	billingStatementModel     model.OnecBillingStatementModel
}

// GeneratorConfig 生成器配置
type GeneratorConfig struct {
	ClusterModel              model.OnecClusterModel
	ProjectModel              model.OnecProjectModel
	ProjectClusterModel       model.OnecProjectClusterModel
	ProjectWorkspaceModel     model.OnecProjectWorkspaceModel
	ProjectApplicationModel   model.OnecProjectApplicationModel
	BillingPriceConfigModel   model.OnecBillingPriceConfigModel
	BillingConfigBindingModel model.OnecBillingConfigBindingModel
	BillingStatementModel     model.OnecBillingStatementModel
}

// NewGenerator 创建账单生成器
func NewGenerator(ctx context.Context, config GeneratorConfig) *Generator {
	return &Generator{
		ctx:                       ctx,
		logger:                    logx.WithContext(ctx),
		calculator:                NewCalculator(),
		clusterModel:              config.ClusterModel,
		projectModel:              config.ProjectModel,
		projectClusterModel:       config.ProjectClusterModel,
		projectWorkspaceModel:     config.ProjectWorkspaceModel,
		projectApplicationModel:   config.ProjectApplicationModel,
		billingPriceConfigModel:   config.BillingPriceConfigModel,
		billingConfigBindingModel: config.BillingConfigBindingModel,
		billingStatementModel:     config.BillingStatementModel,
	}
}

// generateStatementNo 生成账单编号
func (g *Generator) generateStatementNo() string {
	return uuid.New().String()
}

// isNoBillingConfig 判断是否为不计费配置
func (g *Generator) isNoBillingConfig(priceConfigId uint64) bool {
	return priceConfigId == NoBillingPriceConfigId
}

// getPriceConfigForProjectCluster 获取项目集群的价格配置
// 查询优先级：项目集群级别 > 集群默认级别
// 如果项目集群级别没有绑定但集群默认有，则自动创建项目集群级别的绑定
func (g *Generator) getPriceConfigForProjectCluster(clusterUuid string, projectId uint64) (*BindingInfo, *PriceConfig, error) {
	// 先查询项目集群级别的绑定
	binding, err := g.billingConfigBindingModel.FindOneByBindingTypeBindingClusterUuidBindingProjectId(
		g.ctx, BindingTypeProjectCluster, clusterUuid, projectId,
	)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		g.logger.Errorf("查询项目集群级别价格绑定失败, clusterUuid: %s, projectId: %d, err: %v", clusterUuid, projectId, err)
		return nil, nil, fmt.Errorf("查询项目集群级别价格绑定失败: %v", err)
	}

	// 如果项目集群级别有绑定，检查是否为不计费配置
	if binding != nil && !errors.Is(err, model.ErrNotFound) {
		// 检查是否为不计费配置（id=1）
		if g.isNoBillingConfig(binding.PriceConfigId) {
			g.logger.Infof("项目集群绑定了不计费配置，跳过, clusterUuid: %s, projectId: %d, priceConfigId: %d",
				clusterUuid, projectId, binding.PriceConfigId)
			return nil, nil, nil
		}
		return g.buildBindingAndPriceConfig(binding)
	}

	// 项目集群级别没有绑定，查询集群默认级别
	clusterBinding, err := g.billingConfigBindingModel.FindOneByBindingTypeBindingClusterUuidBindingProjectId(
		g.ctx, BindingTypeCluster, clusterUuid, 0,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			g.logger.Infof("集群未配置价格绑定, clusterUuid: %s", clusterUuid)
			return nil, nil, nil
		}
		g.logger.Errorf("查询集群默认价格绑定失败, clusterUuid: %s, err: %v", clusterUuid, err)
		return nil, nil, fmt.Errorf("查询集群默认价格绑定失败: %v", err)
	}

	// 检查集群默认配置是否为不计费配置（id=1）
	if g.isNoBillingConfig(clusterBinding.PriceConfigId) {
		g.logger.Infof("集群绑定了不计费配置，跳过, clusterUuid: %s, priceConfigId: %d",
			clusterUuid, clusterBinding.PriceConfigId)
		return nil, nil, nil
	}

	// 集群有默认配置，自动为项目集群创建绑定记录
	g.logger.Infof("项目集群无绑定，基于集群默认配置创建绑定, clusterUuid: %s, projectId: %d, priceConfigId: %d",
		clusterUuid, projectId, clusterBinding.PriceConfigId)

	newBinding := &model.OnecBillingConfigBinding{
		BindingType:        BindingTypeProjectCluster,
		BindingClusterUuid: clusterUuid,
		BindingProjectId:   projectId,
		PriceConfigId:      clusterBinding.PriceConfigId,
		BillingStartTime:   clusterBinding.BillingStartTime, // 继承集群的计费开始时间
		LastBillingTime:    sql.NullTime{Valid: false},      // 新绑定，上次计费时间为空
		CreatedBy:          "system",
		UpdatedBy:          "system",
		IsDeleted:          0,
	}

	result, err := g.billingConfigBindingModel.Insert(g.ctx, newBinding)
	if err != nil {
		g.logger.Errorf("创建项目集群价格绑定失败, clusterUuid: %s, projectId: %d, err: %v", clusterUuid, projectId, err)
		return nil, nil, fmt.Errorf("创建项目集群价格绑定失败: %v", err)
	}

	// 获取新插入记录的ID
	newId, err := result.LastInsertId()
	if err != nil {
		g.logger.Errorf("获取新绑定ID失败, err: %v", err)
		return nil, nil, fmt.Errorf("获取新绑定ID失败: %v", err)
	}
	newBinding.Id = uint64(newId)

	g.logger.Infof("项目集群价格绑定创建成功, bindingId: %d, clusterUuid: %s, projectId: %d",
		newBinding.Id, clusterUuid, projectId)

	return g.buildBindingAndPriceConfig(newBinding)
}

// buildBindingAndPriceConfig 根据绑定记录构建 BindingInfo 和 PriceConfig
func (g *Generator) buildBindingAndPriceConfig(binding *model.OnecBillingConfigBinding) (*BindingInfo, *PriceConfig, error) {
	// 再次检查是否为不计费配置（双重保险）
	if g.isNoBillingConfig(binding.PriceConfigId) {
		g.logger.Infof("绑定了不计费配置，跳过, bindingId: %d, priceConfigId: %d",
			binding.Id, binding.PriceConfigId)
		return nil, nil, nil
	}

	// 查询价格配置详情
	priceConfig, err := g.billingPriceConfigModel.FindOne(g.ctx, binding.PriceConfigId)
	if err != nil {
		g.logger.Errorf("查询价格配置失败, configId: %d, err: %v", binding.PriceConfigId, err)
		return nil, nil, fmt.Errorf("查询价格配置失败: %v", err)
	}

	bindingInfo := &BindingInfo{
		BindingId:        binding.Id,
		BindingType:      binding.BindingType,
		ClusterUuid:      binding.BindingClusterUuid,
		ProjectId:        binding.BindingProjectId,
		PriceConfigId:    binding.PriceConfigId,
		BillingStartTime: binding.BillingStartTime,
		LastBillingTime:  binding.LastBillingTime,
	}

	price := &PriceConfig{
		ConfigId:      priceConfig.Id,
		CpuPrice:      priceConfig.CpuPrice,
		MemoryPrice:   priceConfig.MemoryPrice,
		StoragePrice:  priceConfig.StoragePrice,
		GpuPrice:      priceConfig.GpuPrice,
		PodPrice:      priceConfig.PodPrice,
		ManagementFee: priceConfig.ManagementFee,
	}

	return bindingInfo, price, nil
}

// getProjectClusterInfo 获取项目集群资源信息
func (g *Generator) getProjectClusterInfo(projectCluster *model.OnecProjectCluster) (*ProjectClusterInfo, error) {
	// 查询项目信息
	project, err := g.projectModel.FindOne(g.ctx, projectCluster.ProjectId)
	if err != nil {
		g.logger.Errorf("查询项目失败, projectId: %d, err: %v", projectCluster.ProjectId, err)
		return nil, fmt.Errorf("查询项目失败: %v", err)
	}

	// 查询集群信息
	cluster, err := g.clusterModel.FindOneByUuid(g.ctx, projectCluster.ClusterUuid)
	if err != nil {
		g.logger.Errorf("查询集群失败, clusterUuid: %s, err: %v", projectCluster.ClusterUuid, err)
		return nil, fmt.Errorf("查询集群失败: %v", err)
	}

	// 转换资源字段
	cpuCapacity, err := utils.CPUToCores(projectCluster.CpuCapacity)
	if err != nil {
		g.logger.Errorf("CPU容量转换失败, value: %s, err: %v", projectCluster.CpuCapacity, err)
		return nil, fmt.Errorf("CPU容量转换失败: %v", err)
	}

	memCapacity, err := utils.MemoryToGiB(projectCluster.MemCapacity)
	if err != nil {
		g.logger.Errorf("内存容量转换失败, value: %s, err: %v", projectCluster.MemCapacity, err)
		return nil, fmt.Errorf("内存容量转换失败: %v", err)
	}

	storageLimit, err := utils.MemoryToGiB(projectCluster.StorageLimit)
	if err != nil {
		g.logger.Errorf("存储配额转换失败, value: %s, err: %v", projectCluster.StorageLimit, err)
		return nil, fmt.Errorf("存储配额转换失败: %v", err)
	}

	gpuCapacity, err := utils.GPUToCount(projectCluster.GpuCapacity)
	if err != nil {
		g.logger.Errorf("GPU容量转换失败, value: %s, err: %v", projectCluster.GpuCapacity, err)
		return nil, fmt.Errorf("GPU容量转换失败: %v", err)
	}

	// 统计工作空间数量
	workspaceCount, err := g.countWorkspaces(projectCluster.Id)
	if err != nil {
		g.logger.Errorf("统计工作空间数量失败, projectClusterId: %d, err: %v", projectCluster.Id, err)
		return nil, fmt.Errorf("统计工作空间数量失败: %v", err)
	}

	// 统计应用数量
	applicationCount, err := g.countApplications(projectCluster.Id)
	if err != nil {
		g.logger.Errorf("统计应用数量失败, projectClusterId: %d, err: %v", projectCluster.Id, err)
		return nil, fmt.Errorf("统计应用数量失败: %v", err)
	}

	return &ProjectClusterInfo{
		ProjectClusterId: projectCluster.Id,
		ProjectId:        project.Id,
		ProjectName:      project.Name,
		ProjectUuid:      project.Uuid,
		ClusterUuid:      cluster.Uuid,
		ClusterName:      cluster.Name,
		CpuCapacity:      cpuCapacity,
		MemCapacity:      memCapacity,
		StorageLimit:     storageLimit,
		GpuCapacity:      gpuCapacity,
		PodsLimit:        projectCluster.PodsLimit,
		WorkspaceCount:   workspaceCount,
		ApplicationCount: applicationCount,
	}, nil
}

// countWorkspaces 统计项目集群下的工作空间数量
func (g *Generator) countWorkspaces(projectClusterId uint64) (int64, error) {
	workspaces, err := g.projectWorkspaceModel.SearchNoPage(
		g.ctx, "", true,
		"`project_cluster_id` = ?", projectClusterId,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return int64(len(workspaces)), nil
}

// countApplications 统计项目集群下的应用数量
func (g *Generator) countApplications(projectClusterId uint64) (int64, error) {
	// 先查询该项目集群下的所有工作空间
	workspaces, err := g.projectWorkspaceModel.SearchNoPage(
		g.ctx, "", true,
		"`project_cluster_id` = ?", projectClusterId,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}

	if len(workspaces) == 0 {
		return 0, nil
	}

	// 统计所有工作空间下的应用数量
	var totalCount int64
	for _, ws := range workspaces {
		apps, err := g.projectApplicationModel.SearchNoPage(
			g.ctx, "", true,
			"`workspace_id` = ?", ws.Id,
		)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return 0, err
		}
		totalCount += int64(len(apps))
	}

	return totalCount, nil
}

// softDeleteStatementsByBinding 软删除绑定关联的所有账单
func (g *Generator) softDeleteStatementsByBinding(bindingId uint64) error {
	g.logger.Infof("开始软删除绑定关联的账单, bindingId: %d", bindingId)

	// 分页查询并删除，避免一次加载过多数据
	pageSize := uint64(100)
	page := uint64(1)
	deletedCount := 0

	for {
		// 查询该绑定关联的账单
		_, statements, total, err := g.billingStatementModel.SearchWithSummary(
			g.ctx, 0, 0, "", 0, "", page, pageSize, "id", true,
		)
		if err != nil {
			// 如果是没有数据的情况，直接返回
			if errors.Is(err, model.ErrNotFound) || total == 0 {
				break
			}
			return fmt.Errorf("查询账单失败: %v", err)
		}

		if len(statements) == 0 {
			break
		}

		// 软删除匹配的账单
		hasMatch := false
		for _, stmt := range statements {
			if stmt.BindingId == bindingId && stmt.IsDeleted == 0 {
				hasMatch = true
				err = g.billingStatementModel.SoftDelete(g.ctx, stmt.Id)
				if err != nil {
					g.logger.Errorf("软删除账单失败, statementId: %d, err: %v", stmt.Id, err)
					continue
				}
				deletedCount++
				g.logger.Infof("软删除账单成功, statementId: %d, statementNo: %s", stmt.Id, stmt.StatementNo)
			}
		}

		// 如果当前页没有匹配的记录且已经是最后一页，退出循环
		if uint64(len(statements)) < pageSize {
			break
		}

		// 如果没有匹配的记录，继续下一页
		if !hasMatch {
			page++
			continue
		}

		// 因为删除后数据会变化，重新从第一页开始
		page = 1
	}

	g.logger.Infof("软删除绑定关联账单完成, bindingId: %d, deletedCount: %d", bindingId, deletedCount)
	return nil
}

// updateBindingLastBillingTime 更新绑定的上次计费时间
func (g *Generator) updateBindingLastBillingTime(bindingId uint64, lastBillingTime time.Time) error {
	binding, err := g.billingConfigBindingModel.FindOne(g.ctx, bindingId)
	if err != nil {
		return fmt.Errorf("查询绑定信息失败: %v", err)
	}

	binding.LastBillingTime = sql.NullTime{
		Time:  lastBillingTime,
		Valid: true,
	}

	err = g.billingConfigBindingModel.Update(g.ctx, binding)
	if err != nil {
		return fmt.Errorf("更新绑定计费时间失败: %v", err)
	}

	return nil
}

// generateSingleStatement 生成单个账单
func (g *Generator) generateSingleStatement(
	projectCluster *model.OnecProjectCluster,
	option *GenerateOption,
) error {
	// 获取价格配置（如果项目集群没有绑定，会自动基于集群默认配置创建）
	bindingInfo, priceConfig, err := g.getPriceConfigForProjectCluster(
		projectCluster.ClusterUuid,
		projectCluster.ProjectId,
	)
	if err != nil {
		return err
	}

	// 没有价格配置，跳过（包括绑定了不计费配置的情况）
	if bindingInfo == nil || priceConfig == nil {
		g.logger.Infof("项目集群未配置价格或配置为不计费，跳过账单生成, projectId: %d, clusterUuid: %s",
			projectCluster.ProjectId, projectCluster.ClusterUuid)
		return nil
	}

	// 获取项目集群资源信息
	info, err := g.getProjectClusterInfo(projectCluster)
	if err != nil {
		return err
	}

	// 确定账单类型和计费周期
	statementType := option.StatementType
	var billingStartTime, billingEndTime time.Time
	billingEndTime = time.Now()

	if !bindingInfo.LastBillingTime.Valid {
		// 上次计费时间为空，首次生成账单，强制类型为 initial
		statementType = StatementTypeInitial
		if bindingInfo.BillingStartTime.Valid {
			billingStartTime = bindingInfo.BillingStartTime.Time
		} else {
			// 如果计费开始时间也为空，使用当前时间
			g.logger.Errorf("绑定缺少计费开始时间, bindingId: %d", bindingInfo.BindingId)
			return fmt.Errorf("绑定缺少计费开始时间")
		}

		// 软删除之前该绑定关联的所有账单
		err = g.softDeleteStatementsByBinding(bindingInfo.BindingId)
		if err != nil {
			g.logger.Errorf("软删除旧账单失败, bindingId: %d, err: %v", bindingInfo.BindingId, err)
		}
	} else {
		// 从上次计费时间开始
		billingStartTime = bindingInfo.LastBillingTime.Time
	}

	// 计算计费周期
	period := g.calculator.CalculateBillingPeriod(billingStartTime, billingEndTime)
	if period.BillingHours <= 0 {
		g.logger.Infof("计费时长为0，跳过账单生成, projectId: %d, clusterUuid: %s",
			projectCluster.ProjectId, projectCluster.ClusterUuid)
		return nil
	}

	// 计算费用
	costDetail := g.calculator.CalculateCost(info, priceConfig, period.BillingHours)

	// 构建账单记录
	statement := &model.OnecBillingStatement{
		StatementNo:        g.generateStatementNo(),
		StatementType:      statementType,
		BillingStartTime:   sql.NullTime{Time: period.StartTime, Valid: true},
		BillingEndTime:     sql.NullTime{Time: period.EndTime, Valid: true},
		BillingHours:       period.BillingHours,
		ClusterUuid:        info.ClusterUuid,
		ClusterName:        info.ClusterName,
		ProjectId:          info.ProjectId,
		ProjectName:        info.ProjectName,
		ProjectUuid:        info.ProjectUuid,
		ProjectClusterId:   info.ProjectClusterId,
		BindingId:          bindingInfo.BindingId,
		CpuCapacity:        g.calculator.FormatCpuCapacity(info.CpuCapacity),
		MemCapacity:        g.calculator.FormatMemoryCapacity(info.MemCapacity),
		StorageLimit:       g.calculator.FormatStorageCapacity(info.StorageLimit),
		GpuCapacity:        g.calculator.FormatGpuCapacity(info.GpuCapacity),
		PodsLimit:          info.PodsLimit,
		WorkspaceCount:     info.WorkspaceCount,
		ApplicationCount:   info.ApplicationCount,
		PriceCpu:           costDetail.PriceCpu,
		PriceMemory:        costDetail.PriceMemory,
		PriceStorage:       costDetail.PriceStorage,
		PriceGpu:           costDetail.PriceGpu,
		PricePod:           costDetail.PricePod,
		PriceManagementFee: costDetail.PriceManagementFee,
		CpuCost:            costDetail.CpuCost,
		MemoryCost:         costDetail.MemoryCost,
		StorageCost:        costDetail.StorageCost,
		GpuCost:            costDetail.GpuCost,
		PodCost:            costDetail.PodCost,
		ManagementFee:      costDetail.ManagementFee,
		ResourceCostTotal:  costDetail.ResourceCostTotal,
		TotalAmount:        costDetail.TotalAmount,
		Remark:             fmt.Sprintf("账单类型: %s", statementType),
		CreatedBy:          option.CreatedBy,
		UpdatedBy:          option.CreatedBy,
		IsDeleted:          0,
	}

	// 插入账单记录
	_, err = g.billingStatementModel.Insert(g.ctx, statement)
	if err != nil {
		g.logger.Errorf("插入账单记录失败, err: %v", err)
		return fmt.Errorf("插入账单记录失败: %v", err)
	}

	// 更新绑定的上次计费时间
	err = g.updateBindingLastBillingTime(bindingInfo.BindingId, period.EndTime)
	if err != nil {
		g.logger.Errorf("更新计费时间失败, bindingId: %d, err: %v", bindingInfo.BindingId, err)
		return fmt.Errorf("更新计费时间失败: %v", err)
	}

	g.logger.Infof("账单生成成功, statementNo: %s, projectId: %d, clusterUuid: %s, totalAmount: %.2f",
		statement.StatementNo, info.ProjectId, info.ClusterUuid, statement.TotalAmount)

	return nil
}

// GenerateAll 生成所有账单
func (g *Generator) GenerateAll(option *GenerateOption) (*GenerateResult, error) {
	g.logger.Info("开始生成所有账单")

	result := &GenerateResult{
		FailedItems: make([]string, 0),
	}

	// 查询所有项目集群关系
	projectClusters, err := g.projectClusterModel.SearchNoPage(g.ctx, "", true, "")
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			g.logger.Info("没有项目集群数据，跳过账单生成")
			return result, nil
		}
		g.logger.Errorf("查询项目集群列表失败, err: %v", err)
		return nil, fmt.Errorf("查询项目集群列表失败: %v", err)
	}

	result.TotalCount = len(projectClusters)

	for _, pc := range projectClusters {
		err = g.generateSingleStatement(pc, option)
		if err != nil {
			result.FailedCount++
			result.FailedItems = append(result.FailedItems,
				fmt.Sprintf("projectId=%d, clusterUuid=%s, err=%v", pc.ProjectId, pc.ClusterUuid, err))
			continue
		}
		result.SuccessCount++
	}

	g.logger.Infof("账单生成完成, total: %d, success: %d, failed: %d",
		result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByCluster 根据集群生成账单
func (g *Generator) GenerateByCluster(clusterUuid string, option *GenerateOption) (*GenerateResult, error) {
	g.logger.Infof("开始生成集群账单, clusterUuid: %s", clusterUuid)

	result := &GenerateResult{
		FailedItems: make([]string, 0),
	}

	// 查询该集群关联的所有项目集群
	projectClusters, err := g.projectClusterModel.SearchNoPage(
		g.ctx, "", true,
		"`cluster_uuid` = ?", clusterUuid,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			g.logger.Infof("集群没有关联项目，跳过账单生成, clusterUuid: %s", clusterUuid)
			return result, nil
		}
		g.logger.Errorf("查询集群关联项目失败, clusterUuid: %s, err: %v", clusterUuid, err)
		return nil, fmt.Errorf("查询集群关联项目失败: %v", err)
	}

	result.TotalCount = len(projectClusters)

	for _, pc := range projectClusters {
		err = g.generateSingleStatement(pc, option)
		if err != nil {
			result.FailedCount++
			result.FailedItems = append(result.FailedItems,
				fmt.Sprintf("projectId=%d, err=%v", pc.ProjectId, err))
			continue
		}
		result.SuccessCount++
	}

	g.logger.Infof("集群账单生成完成, clusterUuid: %s, total: %d, success: %d, failed: %d",
		clusterUuid, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByProject 根据项目生成账单
func (g *Generator) GenerateByProject(projectId uint64, option *GenerateOption) (*GenerateResult, error) {
	g.logger.Infof("开始生成项目账单, projectId: %d", projectId)

	result := &GenerateResult{
		FailedItems: make([]string, 0),
	}

	// 查询该项目关联的所有集群
	projectClusters, err := g.projectClusterModel.SearchNoPage(
		g.ctx, "", true,
		"`project_id` = ?", projectId,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			g.logger.Infof("项目没有关联集群，跳过账单生成, projectId: %d", projectId)
			return result, nil
		}
		g.logger.Errorf("查询项目关联集群失败, projectId: %d, err: %v", projectId, err)
		return nil, fmt.Errorf("查询项目关联集群失败: %v", err)
	}

	result.TotalCount = len(projectClusters)

	for _, pc := range projectClusters {
		err = g.generateSingleStatement(pc, option)
		if err != nil {
			result.FailedCount++
			result.FailedItems = append(result.FailedItems,
				fmt.Sprintf("clusterUuid=%s, err=%v", pc.ClusterUuid, err))
			continue
		}
		result.SuccessCount++
	}

	g.logger.Infof("项目账单生成完成, projectId: %d, total: %d, success: %d, failed: %d",
		projectId, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByPriceConfig 根据价格配置生成账单
func (g *Generator) GenerateByPriceConfig(priceConfigId uint64, option *GenerateOption) (*GenerateResult, error) {
	g.logger.Infof("开始生成价格配置关联账单, priceConfigId: %d", priceConfigId)

	result := &GenerateResult{
		FailedItems: make([]string, 0),
	}

	// 检查是否为不计费配置
	if g.isNoBillingConfig(priceConfigId) {
		g.logger.Infof("价格配置为不计费配置，跳过账单生成, priceConfigId: %d", priceConfigId)
		return result, nil
	}

	// 查询该价格配置关联的所有绑定
	bindings, err := g.billingConfigBindingModel.FindByPriceConfigId(g.ctx, priceConfigId)
	if err != nil {
		g.logger.Errorf("查询价格配置绑定失败, priceConfigId: %d, err: %v", priceConfigId, err)
		return nil, fmt.Errorf("查询价格配置绑定失败: %v", err)
	}

	if len(bindings) == 0 {
		g.logger.Infof("价格配置没有绑定关系，跳过账单生成, priceConfigId: %d", priceConfigId)
		return result, nil
	}

	// 用于去重，避免重复生成同一个项目集群的账单
	processedKey := make(map[string]bool)

	for _, binding := range bindings {
		var projectClusters []*model.OnecProjectCluster

		if binding.BindingType == BindingTypeCluster {
			// 集群级别绑定，查询该集群下所有项目集群
			// 注意：由于现在会自动创建项目集群级别绑定，这里需要特殊处理
			allProjectClusters, err := g.projectClusterModel.SearchNoPage(
				g.ctx, "", true,
				"`cluster_uuid` = ?", binding.BindingClusterUuid,
			)
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				g.logger.Errorf("查询集群关联项目失败, clusterUuid: %s, err: %v", binding.BindingClusterUuid, err)
				continue
			}

			// 过滤掉已有项目集群级别绑定的（它们会单独处理）
			for _, pc := range allProjectClusters {
				existBinding, _ := g.billingConfigBindingModel.FindOneByBindingTypeBindingClusterUuidBindingProjectId(
					g.ctx, BindingTypeProjectCluster, pc.ClusterUuid, pc.ProjectId,
				)
				// 如果已有项目集群级别绑定，跳过（会在后续循环中处理）
				if existBinding != nil {
					continue
				}
				projectClusters = append(projectClusters, pc)
			}
		} else {
			// 项目集群级别绑定
			pc, err := g.projectClusterModel.FindOneByClusterUuidProjectId(
				g.ctx, binding.BindingClusterUuid, binding.BindingProjectId,
			)
			if err != nil {
				if !errors.Is(err, model.ErrNotFound) {
					g.logger.Errorf("查询项目集群失败, clusterUuid: %s, projectId: %d, err: %v",
						binding.BindingClusterUuid, binding.BindingProjectId, err)
				}
				continue
			}
			projectClusters = append(projectClusters, pc)
		}

		for _, pc := range projectClusters {
			// 去重检查
			key := fmt.Sprintf("%d_%s", pc.ProjectId, pc.ClusterUuid)
			if processedKey[key] {
				continue
			}
			processedKey[key] = true

			result.TotalCount++
			err = g.generateSingleStatement(pc, option)
			if err != nil {
				result.FailedCount++
				result.FailedItems = append(result.FailedItems,
					fmt.Sprintf("projectId=%d, clusterUuid=%s, err=%v", pc.ProjectId, pc.ClusterUuid, err))
				continue
			}
			result.SuccessCount++
		}
	}

	g.logger.Infof("价格配置账单生成完成, priceConfigId: %d, total: %d, success: %d, failed: %d",
		priceConfigId, result.TotalCount, result.SuccessCount, result.FailedCount)

	return result, nil
}

// GenerateByProjectCluster 根据项目集群ID生成单个账单
func (g *Generator) GenerateByProjectCluster(projectClusterId uint64, option *GenerateOption) error {
	g.logger.Infof("开始生成单个项目集群账单, projectClusterId: %d", projectClusterId)

	projectCluster, err := g.projectClusterModel.FindOne(g.ctx, projectClusterId)
	if err != nil {
		g.logger.Errorf("查询项目集群失败, projectClusterId: %d, err: %v", projectClusterId, err)
		return fmt.Errorf("查询项目集群失败: %v", err)
	}

	return g.generateSingleStatement(projectCluster, option)
}

// GenerateByClusterAndProject 根据集群UUID和项目ID生成单个账单
func (g *Generator) GenerateByClusterAndProject(clusterUuid string, projectId uint64, option *GenerateOption) error {
	g.logger.Infof("开始生成项目集群账单, clusterUuid: %s, projectId: %d", clusterUuid, projectId)

	projectCluster, err := g.projectClusterModel.FindOneByClusterUuidProjectId(g.ctx, clusterUuid, projectId)
	if err != nil {
		g.logger.Errorf("查询项目集群失败, clusterUuid: %s, projectId: %d, err: %v", clusterUuid, projectId, err)
		return fmt.Errorf("查询项目集群失败: %v", err)
	}

	return g.generateSingleStatement(projectCluster, option)
}
