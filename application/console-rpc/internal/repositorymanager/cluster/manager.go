package cluster

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/operator"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

// HarborManager Harbor 多实例管理器
type HarborManager struct {
	mu              sync.RWMutex
	clients         map[string]types.HarborClient // uuid -> client
	log             logx.Logger
	ctx             context.Context
	RepositoryModel repository.ContainerRegistryModel
}

// NewHarborManager 创建 Harbor 管理器
func NewHarborManager(repositoryModel repository.ContainerRegistryModel) *HarborManager {
	ctx := context.Background()
	return &HarborManager{
		clients:         make(map[string]types.HarborClient),
		log:             logx.WithContext(ctx),
		ctx:             ctx,
		RepositoryModel: repositoryModel,
	}
}

// Add 添加 Harbor 实例
func (m *HarborManager) Add(config *types.HarborConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[config.UUID]; exists {
		m.log.Infof("Harbor 客户端已存在，将更新: uuid=%s", config.UUID)
		if err := m.clients[config.UUID].Close(); err != nil {
			m.log.Errorf("关闭旧客户端失败: %v", err)
		}
		delete(m.clients, config.UUID)
	}

	client, err := operator.NewHarborClient(m.ctx, config)
	if err != nil {
		m.log.Errorf("创建 Harbor 客户端失败: uuid=%s, error=%v", config.UUID, err)
		return fmt.Errorf("创建 Harbor 客户端失败: %v", err)
	}

	if err := client.Ping(); err != nil {
		client.Close()
		m.log.Errorf("连接 Harbor 失败: uuid=%s, error=%v", config.UUID, err)
		return fmt.Errorf("连接 Harbor 失败: %v", err)
	}

	m.clients[config.UUID] = client
	m.log.Infof("Harbor 客户端添加成功: uuid=%s, endpoint=%s", config.UUID, config.Endpoint)

	return nil
}

// Delete 删除 Harbor 实例
func (m *HarborManager) Delete(uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[uuid]
	if !exists {
		m.log.Infof("Harbor 客户端不存在，无需删除: uuid=%s", uuid)
		return nil
	}

	if err := client.Close(); err != nil {
		m.log.Errorf("关闭 Harbor 客户端失败: uuid=%s, error=%v", uuid, err)
	}

	delete(m.clients, uuid)
	m.log.Infof("Harbor 客户端删除成功: uuid=%s", uuid)

	return nil
}

// Get 获取 Harbor 客户端，如果不存在则从数据库加载
func (m *HarborManager) Get(uuid string) (types.HarborClient, error) {
	// 先尝试从缓存获取
	m.mu.RLock()
	client, exists := m.clients[uuid]
	m.mu.RUnlock()

	if exists {
		m.log.Debugf("从缓存获取 Harbor 客户端: uuid=%s", uuid)
		return client, nil
	}

	// 缓存中不存在，从数据库加载
	m.log.Infof("缓存中不存在，从数据库加载 Harbor: uuid=%s", uuid)

	repo, err := m.RepositoryModel.FindOneByUuid(m.ctx, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			m.log.Errorf("数据库中不存在该 Harbor: uuid=%s", uuid)
			return nil, fmt.Errorf("harbor 不存在: %s", uuid)
		}
		m.log.Errorf("查询数据库失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("查询数据库失败: %v", err)
	}

	// 构建配置
	config := &types.HarborConfig{
		UUID:     repo.Uuid,
		Name:     repo.Name,
		Endpoint: repo.Url,
		Username: repo.Username,
		Password: repo.Password,
		Insecure: repo.Insecure == 1,
		CACert:   repo.CaCert.String,
	}

	// 添加到管理器
	if err := m.Add(config); err != nil {
		m.log.Errorf("添加 Harbor 客户端失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("添加 Harbor 客户端失败: %v", err)
	}

	// 再次从缓存获取
	m.mu.RLock()
	client, exists = m.clients[uuid]
	m.mu.RUnlock()

	if !exists {
		m.log.Errorf("添加后仍无法获取客户端: uuid=%s", uuid)
		return nil, fmt.Errorf("获取客户端失败")
	}

	m.log.Infof("成功从数据库加载并创建 Harbor 客户端: uuid=%s", uuid)
	return client, nil
}

// GetByID 通过数据库 ID 获取 Harbor 客户端
func (m *HarborManager) GetByID(id uint64) (types.HarborClient, error) {
	repo, err := m.RepositoryModel.FindOne(m.ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			m.log.Errorf("数据库中不存在该 Harbor: id=%d", id)
			return nil, fmt.Errorf("Harbor 不存在: id=%d", id)
		}
		m.log.Errorf("查询数据库失败: id=%d, error=%v", id, err)
		return nil, fmt.Errorf("查询数据库失败: %v", err)
	}

	return m.Get(repo.Uuid)
}

// List 列出所有 Harbor 实例的 UUID
func (m *HarborManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uuids := make([]string, 0, len(m.clients))
	for uuid := range m.clients {
		uuids = append(uuids, uuid)
	}

	return uuids
}

// Count 返回 Harbor 实例数量
func (m *HarborManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// Update 更新 Harbor 实例配置
func (m *HarborManager) Update(config *types.HarborConfig) error {
	if err := m.Delete(config.UUID); err != nil {
		return fmt.Errorf("删除旧客户端失败: %v", err)
	}

	if err := m.Add(config); err != nil {
		return fmt.Errorf("添加新客户端失败: %v", err)
	}

	m.log.Infof("更新 Harbor 客户端成功: uuid=%s", config.UUID)
	return nil
}

// Reload 重新加载 Harbor 实例（从数据库）
func (m *HarborManager) Reload(uuid string) error {
	if err := m.Delete(uuid); err != nil {
		m.log.Errorf("删除旧客户端失败: uuid=%s, error=%v", uuid, err)
	}

	_, err := m.Get(uuid)
	if err != nil {
		m.log.Errorf("重新加载客户端失败: uuid=%s, error=%v", uuid, err)
		return fmt.Errorf("重新加载客户端失败: %v", err)
	}

	m.log.Infof("重新加载 Harbor 客户端成功: uuid=%s", uuid)
	return nil
}

// Close 关闭所有客户端
func (m *HarborManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for uuid, client := range m.clients {
		if err := client.Close(); err != nil {
			m.log.Errorf("关闭 Harbor 客户端失败: uuid=%s, error=%v", uuid, err)
			lastErr = err
		}
	}

	m.clients = make(map[string]types.HarborClient)
	m.log.Info("所有 Harbor 客户端已关闭")
	return lastErr
}

// SearchImagesInRepository 在单个仓库中搜索镜像
func (m *HarborManager) SearchImagesInRepository(uuid, imageName string) ([]types.RepositorySearchResult, error) {
	m.log.Infof("在仓库中搜索镜像: uuid=%s, imageName=%s", uuid, imageName)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	results, err := client.SearchImages(imageName)
	if err != nil {
		m.log.Errorf("搜索镜像失败: uuid=%s, error=%v", uuid, err)
		return nil, fmt.Errorf("搜索镜像失败: %v", err)
	}

	m.log.Infof("搜索完成: 找到 %d 个匹配的仓库", len(results))
	return results, nil
}

// SearchImagesGlobal 全局搜索镜像（跨所有 Harbor）- 修复版本
func (m *HarborManager) SearchImagesGlobal(imageName string) ([]types.ImageSearchResult, error) {
	m.log.Infof("开始全局搜索镜像: %s", imageName)

	// 获取所有 Harbor 仓库 - 使用 SearchNoPage 替代不存在的 FindAll
	repos, err := m.RepositoryModel.SearchNoPage(m.ctx, "id", true, "", nil)
	if err != nil {
		m.log.Errorf("获取仓库列表失败: %v", err)
		return nil, fmt.Errorf("获取仓库列表失败: %v", err)
	}

	var results []types.ImageSearchResult

	for _, repo := range repos {
		// 获取或创建客户端
		client, err := m.Get(repo.Uuid)
		if err != nil {
			m.log.Errorf("获取 Harbor 客户端失败: uuid=%s, error=%v", repo.Uuid, err)
			continue
		}

		m.log.Infof("在 Harbor %s 中搜索镜像", repo.Uuid)

		// 搜索镜像
		repoResults, err := client.SearchImages(imageName)
		if err != nil {
			m.log.Errorf("搜索失败: uuid=%s, error=%v", repo.Uuid, err)
			continue
		}

		if len(repoResults) > 0 {
			results = append(results, types.ImageSearchResult{
				Name:       client.GetName(),
				Endpoint:   client.GetEndpoint(),
				UUID:       repo.Uuid,
				Repository: repoResults,
			})
		}
	}

	m.log.Infof("全局搜索完成: 找到 %d 个 Harbor 包含匹配的镜像", len(results))
	return results, nil
}

// parsePublicStatus 从 metadata 中解析公开状态
func parsePublicStatus(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}

	// Harbor 2.0 中 public 存储在 metadata 中，值为字符串 "true" 或 "false"
	publicStr, exists := metadata["public"]
	if !exists {
		return false
	}

	return strings.ToLower(publicStr) == "true"
}

// SearchPublicImages 搜索公开镜像
func (m *HarborManager) SearchPublicImages(uuid, imageName string) ([]types.RepositorySearchResult, error) {
	m.log.Infof("搜索公开镜像: uuid=%s, imageName=%s", uuid, imageName)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	var results []types.RepositorySearchResult
	searchLower := strings.ToLower(imageName)

	// 获取所有项目
	projectOp := client.Project()
	projectResp, err := projectOp.List(types.ListRequest{
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		m.log.Errorf("获取项目列表失败: %v", err)
		return nil, fmt.Errorf("获取项目列表失败: %v", err)
	}

	// 只搜索公开项目
	repoOp := client.Repository()
	artifactOp := client.Artifact()

	for _, project := range projectResp.Items {
		// 修正公开状态判断：从 metadata 中读取
		isPublic := parsePublicStatus(project.Metadata)
		m.log.Debugf("项目 %s 公开状态: %v (metadata: %v)", project.Name, isPublic, project.Metadata)

		if !isPublic {
			continue // 跳过非公开项目
		}

		repoResp, err := repoOp.List(project.Name, types.ListRequest{
			Page:     1,
			PageSize: 100,
		})
		if err != nil {
			m.log.Errorf("获取仓库列表失败: project=%s, error=%v", project.Name, err)
			continue
		}

		for _, repo := range repoResp.Items {
			repoNameLower := strings.ToLower(repo.Name)
			if strings.Contains(repoNameLower, searchLower) {
				artifactResp, err := artifactOp.List(project.Name, repo.Name, types.ListRequest{
					Page:     1,
					PageSize: 100,
				})
				if err != nil {
					m.log.Errorf("获取制品列表失败: repo=%s, error=%v", repo.Name, err)
					continue
				}

				var tags []string
				tagSet := make(map[string]bool)

				for _, artifact := range artifactResp.Items {
					for _, tag := range artifact.Tags {
						if !tagSet[tag.Name] {
							tags = append(tags, tag.Name)
							tagSet[tag.Name] = true
						}
					}
				}

				if len(tags) > 0 {
					results = append(results, types.RepositorySearchResult{
						ProjectName: project.Name,
						RepoName:    repo.Name,
						Tags:        tags,
					})
				}
			}
		}
	}

	m.log.Infof("公开镜像搜索完成: 找到 %d 个匹配的仓库", len(results))
	return results, nil
}

// SearchImagesByProjects 在指定项目中搜索镜像
func (m *HarborManager) SearchImagesByProjects(uuid string, projectNames []string, imageName string) ([]types.RepositorySearchResult, error) {
	m.log.Infof("在指定项目中搜索镜像: uuid=%s, projects=%v, imageName=%s", uuid, projectNames, imageName)

	client, err := m.Get(uuid)
	if err != nil {
		return nil, err
	}

	var results []types.RepositorySearchResult
	searchLower := strings.ToLower(imageName)
	projectSet := make(map[string]bool)

	for _, pn := range projectNames {
		projectSet[pn] = true
	}

	repoOp := client.Repository()
	artifactOp := client.Artifact()

	for projectName := range projectSet {
		repoResp, err := repoOp.List(projectName, types.ListRequest{
			Page:     1,
			PageSize: 100,
		})
		if err != nil {
			m.log.Errorf("获取仓库列表失败: project=%s, error=%v", projectName, err)
			continue
		}

		for _, repo := range repoResp.Items {
			repoNameLower := strings.ToLower(repo.Name)
			if strings.Contains(repoNameLower, searchLower) {
				artifactResp, err := artifactOp.List(projectName, repo.Name, types.ListRequest{
					Page:     1,
					PageSize: 100,
				})
				if err != nil {
					m.log.Errorf("获取制品列表失败: repo=%s, error=%v", repo.Name, err)
					continue
				}

				var tags []string
				tagSet := make(map[string]bool)

				for _, artifact := range artifactResp.Items {
					for _, tag := range artifact.Tags {
						if !tagSet[tag.Name] {
							tags = append(tags, tag.Name)
							tagSet[tag.Name] = true
						}
					}
				}

				if len(tags) > 0 {
					results = append(results, types.RepositorySearchResult{
						ProjectName: projectName,
						RepoName:    repo.Name,
						Tags:        tags,
					})
				}
			}
		}
	}

	m.log.Infof("项目镜像搜索完成: 找到 %d 个匹配的仓库", len(results))
	return results, nil
}
