package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出文件和目录
func NewPodFileListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileListLogic {
	return &PodFileListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileListLogic) PodFileList(req *types.PodFileListReq) (resp *types.PodFileListResp, err error) {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 如果没有指定容器，获取默认容器
	containerName := req.Container
	if containerName == "" {
		containerName, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
		if err != nil {
			l.Errorf("获取默认容器失败: %v", err)
			return nil, fmt.Errorf("获取默认容器失败")
		}
	}

	// 5. 设置默认路径
	path := req.Path
	if path == "" {
		path = "/"
	}

	// 6. 构建文件列表选项
	opts := &k8stypes.FileListOptions{
		ShowHidden: req.ShowHidden,
		SortBy:     k8stypes.FileSortBy(req.SortBy),
		SortDesc:   req.SortDesc,
		Search:     req.Search,
		FileTypes:  req.FileTypes,
		MaxDepth:   req.MaxDepth,
		Recursive:  req.Recursive,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	// 7. 获取文件列表
	result, err := podClient.ListFiles(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, path, opts)
	if err != nil {
		l.Errorf("获取文件列表失败: %v", err)
		return nil, fmt.Errorf("获取文件列表失败: %v", err)
	}

	// 8. 转换响应格式
	resp = &types.PodFileListResp{
		Files:       convertFileInfoList(result.Files),
		CurrentPath: result.CurrentPath,
		Breadcrumbs: convertBreadcrumbs(result.Breadcrumbs),
		TotalCount:  result.TotalCount,
		TotalSize:   result.TotalSize,
		Container:   containerName,
		HasMore:     result.HasMore,
	}

	l.Infof("成功获取文件列表，路径: %s, 文件数: %d, 总大小: %d", path, len(resp.Files), resp.TotalSize)

	return resp, nil
}

// convertFileInfoList 转换文件信息列表
func convertFileInfoList(files []k8stypes.FileInfo) []types.FileInfo {
	result := make([]types.FileInfo, 0, len(files))
	for _, f := range files {
		result = append(result, types.FileInfo{
			Name:         f.Name,
			Path:         f.Path,
			IsDir:        f.IsDir,
			Size:         f.Size,
			Mode:         f.Mode,
			ModTime:      f.ModTime.Format("2006-01-02T15:04:05Z07:00"), // RFC3339 格式
			Owner:        f.Owner,
			Group:        f.Group,
			IsLink:       f.IsLink,
			LinkTarget:   f.LinkTarget,
			IsReadable:   f.IsReadable,
			IsWritable:   f.IsWritable,
			IsExecutable: f.IsExecutable,
			MimeType:     f.MimeType,
			Children:     f.Children,
			Permissions: types.FilePermission{
				User: types.PermissionBits{
					Read:    f.Permissions.User.Read,
					Write:   f.Permissions.User.Write,
					Execute: f.Permissions.User.Execute,
				},
				Group: types.PermissionBits{
					Read:    f.Permissions.Group.Read,
					Write:   f.Permissions.Group.Write,
					Execute: f.Permissions.Group.Execute,
				},
				Other: types.PermissionBits{
					Read:    f.Permissions.Other.Read,
					Write:   f.Permissions.Other.Write,
					Execute: f.Permissions.Other.Execute,
				},
			},
		})
	}
	return result
}

// convertBreadcrumbs 转换面包屑导航
func convertBreadcrumbs(breadcrumbs []k8stypes.BreadcrumbItem) []types.BreadcrumbItem {
	result := make([]types.BreadcrumbItem, 0, len(breadcrumbs))
	for _, b := range breadcrumbs {
		result = append(result, types.BreadcrumbItem{
			Name: b.Name,
			Path: b.Path,
		})
	}
	return result
}
