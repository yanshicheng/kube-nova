package operator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
)

type HarborClientImpl struct {
	*BaseOperator
	uuid     string
	name     string
	endpoint string
}

func NewHarborClient(ctx context.Context, config *types.HarborConfig) (types.HarborClient, error) {
	if config.UUID == "" || config.Endpoint == "" || config.Username == "" || config.Password == "" {
		return nil, fmt.Errorf("配置无效: uuid、endpoint、username、password 不能为空")
	}

	base, err := NewBaseOperator(ctx, config.Endpoint, config.Username, config.Password, config.Insecure, config.CACert)
	if err != nil {
		return nil, fmt.Errorf("创建基础操作器失败: %w", err)
	}

	return &HarborClientImpl{
		BaseOperator: base,
		uuid:         config.UUID,
		name:         config.Name,
		endpoint:     config.Endpoint,
	}, nil
}

func (h *HarborClientImpl) GetUUID() string {
	return h.uuid
}

func (h *HarborClientImpl) GetName() string {
	return h.name
}

func (h *HarborClientImpl) GetEndpoint() string {
	return h.endpoint
}

// parsePublicStatus 从 metadata 中解析公开状态
func parsePublicStatusFromMetadata(metadata map[string]string) bool {
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

func (h *HarborClientImpl) SystemInfo() (*types.SystemInfo, error) {
	h.log.Info("获取系统信息")

	info := &types.SystemInfo{}

	// 1. 从 systeminfo API 获取存储和统计信息
	var sysInfoRaw map[string]interface{}
	if err := h.doRequest("GET", "/api/v2.0/systeminfo", nil, &sysInfoRaw); err != nil {
		h.log.Errorf("获取系统信息失败: %v", err)
	} else {
		h.log.Infof("SystemInfo 原始数据: %+v", sysInfoRaw)

		// 解析存储信息
		if storage, ok := sysInfoRaw["storage"].(map[string]interface{}); ok {
			if total, ok := storage["total"].(float64); ok {
				info.StorageTotal = int64(total)
			}
			if free, ok := storage["free"].(float64); ok {
				info.StorageFree = int64(free)
			}
		}
	}

	// 2. 使用 statistics API 获取项目和仓库统计（Harbor 2.0+）
	var statistics map[string]interface{}
	if err := h.doRequest("GET", "/api/v2.0/statistics", nil, &statistics); err != nil {
		h.log.Errorf("获取统计信息失败: %v", err)
	} else {
		h.log.Infof("Statistics 数据: %+v", statistics)

		// 解析项目统计
		if totalProjects, ok := statistics["total_project_count"].(float64); ok {
			info.TotalProjects = int64(totalProjects)
		}
		if publicProjects, ok := statistics["public_project_count"].(float64); ok {
			info.PublicProjects = int64(publicProjects)
		}
		if privateProjects, ok := statistics["private_project_count"].(float64); ok {
			info.PrivateProjects = int64(privateProjects)
		}

		// 解析仓库统计
		if totalRepos, ok := statistics["total_repo_count"].(float64); ok {
			info.TotalRepositories = int64(totalRepos)
		}
		if publicRepos, ok := statistics["public_repo_count"].(float64); ok {
			info.PublicRepositories = int64(publicRepos)
		}
		if privateRepos, ok := statistics["private_repo_count"].(float64); ok {
			info.PrivateRepositories = int64(privateRepos)
		}

		// 解析存储使用（如果 statistics 中有）
		if storage, ok := statistics["total_storage_consumption"].(float64); ok {
			info.StorageUsed = int64(storage)
		}
	}

	// 3. 如果统计 API 不可用，回退到获取项目列表（仅第一页，估算）
	if info.TotalProjects == 0 {
		h.log.Info("Statistics API 不可用，使用项目列表估算...")

		projectOp := h.Project()
		projectResp, err := projectOp.List(types.ListRequest{
			Page:     1,
			PageSize: 1,
		})
		if err == nil && len(projectResp.Items) > 0 {
			// 简单估算：假设有项目
			info.TotalProjects = int64(projectResp.Total)
			h.log.Infof("估算项目数: %d", info.TotalProjects)
		}
	}

	// 4. 计算已用存储（如果没有从 statistics 获取到）
	if info.StorageUsed == 0 && info.StorageTotal > 0 && info.StorageFree > 0 {
		info.StorageUsed = info.StorageTotal - info.StorageFree
	}

	// 5. 如果仍然缺少数据，从配额系统获取
	if info.StorageUsed == 0 || info.StorageTotal == 0 {
		h.log.Info("从配额系统获取存储信息...")

		quotaOp := h.Quota()
		// 获取前 10 个项目的配额（避免性能问题）
		quotas, err := quotaOp.List("project", "")
		if err == nil && len(quotas) > 0 {
			var totalStorage, usedStorage int64
			for i, quota := range quotas {
				if i >= 10 { // 限制只统计前 10 个
					break
				}
				if quota.Hard.Storage > 0 {
					totalStorage += quota.Hard.Storage
				}
				usedStorage += quota.Used.Storage
			}

			if totalStorage > 0 {
				info.StorageTotal = totalStorage
				info.StorageUsed = usedStorage
				info.StorageFree = totalStorage - usedStorage
				h.log.Infof("从配额统计存储: total=%d, used=%d, free=%d",
					totalStorage, usedStorage, info.StorageFree)
			}
		}
	}

	h.log.Infof("系统信息统计完成: 项目=%d(公开=%d,私有=%d), 仓库=%d(公开=%d,私有=%d), 存储=%d/%d",
		info.TotalProjects, info.PublicProjects, info.PrivateProjects,
		info.TotalRepositories, info.PublicRepositories, info.PrivateRepositories,
		info.StorageUsed, info.StorageTotal)

	return info, nil
}

func (h *HarborClientImpl) Project() types.ProjectOperator {
	return NewProjectOperator(h.ctx, h.BaseOperator)
}

func (h *HarborClientImpl) Repository() types.RepositoryOperator {
	return NewRepositoryOperator(h.ctx, h.BaseOperator)
}

func (h *HarborClientImpl) Artifact() types.ArtifactOperator {
	return NewArtifactOperator(h.ctx, h.BaseOperator)
}

func (h *HarborClientImpl) Quota() types.QuotaOperator {
	return NewQuotaOperator(h.ctx, h.BaseOperator)
}

func (h *HarborClientImpl) Member() types.MemberOperator {
	return NewMemberOperator(h.ctx, h.BaseOperator)
}

func (h *HarborClientImpl) Ping() error {
	h.log.Info("测试连接")

	url := h.endpoint + "/api/v2.0/ping"
	req, err := http.NewRequestWithContext(h.ctx, "GET", url, nil)
	if err != nil {
		h.log.Errorf("创建 Ping 请求失败: %v", err)
		return fmt.Errorf("创建 Ping 请求失败")
	}

	req.SetBasicAuth(h.username, h.password)
	req.Header.Set("Accept", "text/plain")

	resp, err := h.client.Do(req)
	if err != nil {
		h.log.Errorf("Ping 请求失败: %v", err)
		return fmt.Errorf("ping 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.log.Errorf("Ping 失败: status=%d, body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("ping 失败: status=%d", resp.StatusCode)
	}

	h.log.Info("连接测试成功")
	return nil
}

func (h *HarborClientImpl) Close() error {
	h.log.Info("关闭客户端")
	h.client.CloseIdleConnections()
	return nil
}

func (h *HarborClientImpl) SearchImages(imageName string) ([]types.RepositorySearchResult, error) {
	h.log.Infof("搜索镜像: %s", imageName)

	var results []types.RepositorySearchResult
	searchLower := strings.ToLower(imageName)

	// 1. 获取所有项目
	projectOp := h.Project()
	projectResp, err := projectOp.List(types.ListRequest{
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		h.log.Errorf("获取项目列表失败: %v", err)
		return nil, fmt.Errorf("获取项目列表失败: %w", err)
	}

	h.log.Infof("找到 %d 个项目，开始搜索...", projectResp.Total)

	// 2. 遍历每个项目的仓库
	repoOp := h.Repository()
	for _, project := range projectResp.Items {
		repoResp, err := repoOp.List(project.Name, types.ListRequest{
			Page:     1,
			PageSize: 100,
		})
		if err != nil {
			h.log.Errorf("获取项目 %s 的仓库列表失败: %v", project.Name, err)
			continue
		}

		// 3. 检查仓库名称是否匹配
		for _, repo := range repoResp.Items {
			repoNameLower := strings.ToLower(repo.Name)

			// 模糊匹配：仓库名包含搜索关键词
			if strings.Contains(repoNameLower, searchLower) {
				h.log.Infof("匹配到仓库: %s", repo.Name)

				// 4. 获取该仓库的所有标签
				artifactOp := h.Artifact()
				artifactResp, err := artifactOp.List(project.Name, repo.Name, types.ListRequest{
					Page:     1,
					PageSize: 100,
				})
				if err != nil {
					h.log.Errorf("获取仓库 %s 的制品列表失败: %v", repo.Name, err)
					continue
				}

				// 5. 收集所有标签（去重）
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

				// 只有当仓库有标签时才添加到结果
				if len(tags) > 0 {
					results = append(results, types.RepositorySearchResult{
						ProjectName: project.Name,
						RepoName:    repo.Name,
						Tags:        tags,
					})
					h.log.Infof("仓库 %s 包含 %d 个标签", repo.Name, len(tags))
				}
			}
		}
	}

	h.log.Infof("搜索完成: 找到 %d 个匹配的仓库", len(results))
	return results, nil
}

// GetUploadCredential 获取镜像上传凭证
func (h *HarborClientImpl) GetUploadCredential(projectName string) (*types.UploadCredential, error) {
	h.log.Infof("获取上传凭证: project=%s", projectName)

	// 1. 验证项目是否存在
	projectOp := h.Project()
	project, err := projectOp.Get(projectName)
	if err != nil {
		h.log.Errorf("项目不存在: %v", err)
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	h.log.Infof("项目验证成功: %s (ID: %d)", project.Name, project.ProjectID)

	// 2. 处理 endpoint，移除协议前缀
	registryUrl := strings.TrimPrefix(h.endpoint, "https://")
	registryUrl = strings.TrimPrefix(registryUrl, "http://")

	// 3. 构建完整镜像路径
	fullPath := fmt.Sprintf("%s/%s/", registryUrl, projectName)

	// 4. 构建命令示例
	loginCmd := fmt.Sprintf("docker login %s -u %s -p ******", registryUrl, h.username)
	pushCmd := fmt.Sprintf("# 1. 标记镜像\ndocker tag <local-image>:<tag> %s<image-name>:<tag>\n\n# 2. 推送镜像\ndocker push %s<image-name>:<tag>", fullPath, fullPath)

	credential := &types.UploadCredential{
		RegistryUrl:        registryUrl,
		Username:           h.username,
		Password:           h.password,
		FullImagePath:      fullPath,
		DockerLoginCommand: loginCmd,
		DockerPushCommand:  pushCmd,
	}

	h.log.Infof("生成上传凭证成功: registry=%s, project=%s", registryUrl, projectName)
	return credential, nil
}

// GetPullCommand 获取镜像拉取命令
func (h *HarborClientImpl) GetPullCommand(projectName, repoName, tag string) (*types.PullCommand, error) {
	h.log.Infof("获取拉取命令: project=%s, repo=%s, tag=%s", projectName, repoName, tag)

	// 1. 验证制品是否存在
	artifactOp := h.Artifact()
	_, err := artifactOp.Get(projectName, repoName, tag)
	if err != nil {
		h.log.Errorf("制品不存在: %v", err)
		return nil, fmt.Errorf("制品不存在: %w", err)
	}

	// 2. 处理 endpoint，移除协议前缀
	registryUrl := strings.TrimPrefix(h.endpoint, "https://")
	registryUrl = strings.TrimPrefix(registryUrl, "http://")

	// 3. 构建完整镜像 URL
	fullImageUrl := fmt.Sprintf("%s/%s/%s:%s", registryUrl, projectName, repoName, tag)

	// 4. 构建命令
	loginCmd := fmt.Sprintf("docker login %s -u %s -p ******", registryUrl, h.username)
	pullCmd := fmt.Sprintf("docker pull %s", fullImageUrl)

	command := &types.PullCommand{
		RegistryUrl:        registryUrl,
		Username:           h.username,
		Password:           h.password,
		FullImageUrl:       fullImageUrl,
		DockerLoginCommand: loginCmd,
		DockerPullCommand:  pullCmd,
	}

	h.log.Infof("生成拉取命令成功: %s", fullImageUrl)
	return command, nil
}

func (h *HarborClientImpl) User() types.UserOperator {
	return NewUserOperator(h.ctx, h.BaseOperator)
}

// GC 返回垃圾回收操作器
func (h *HarborClientImpl) GC() types.GCOperator {
	return NewGCOperator(h.ctx, h.BaseOperator)
}

// Retention 返回保留策略操作器
func (h *HarborClientImpl) Retention() types.RetentionOperator {
	return NewRetentionOperator(h.ctx, h.BaseOperator)
}

// Replication 返回复制策略操作器
func (h *HarborClientImpl) Replication() types.ReplicationOperator {
	return NewReplicationOperator(h.ctx, h.BaseOperator)
}
