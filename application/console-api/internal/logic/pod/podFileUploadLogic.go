package pod

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	r      *http.Request
}

// 上传文件
func NewPodFileUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext, r *http.Request) *PodFileUploadLogic {
	return &PodFileUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		r:      r,
	}
}

func (l *PodFileUploadLogic) PodFileUpload(req *types.PodFileUploadReq) (resp *types.PodFileUploadResp, err error) {
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

	// 5. 从请求中获取上传的文件
	file, header, err := l.r.FormFile("file")
	if err != nil {
		l.Errorf("获取上传文件失败: %v", err)
		return nil, fmt.Errorf("获取上传文件失败: %v", err)
	}
	defer file.Close()

	// 6. 确定文件名
	filename := req.FileName
	if filename == "" {
		filename = header.Filename
	}

	destPath := filepath.Join(req.Path, filename)

	l.Infof("上传文件: path=%s, fileName=%s, destPath=%s",
		req.Path, filename, destPath)

	// 8. 计算文件大小和校验和
	h := md5.New()
	fileSize, err := io.Copy(h, file)
	if err != nil {
		return nil, fmt.Errorf("计算文件校验和失败: %v", err)
	}
	checksum := fmt.Sprintf("%x", h.Sum(nil))

	// 重置文件指针
	file.Seek(0, 0)

	// 9. 构建上传选项
	uploadOpts := &k8stypes.UploadOptions{
		FileName:    filename,
		FileSize:    fileSize,
		FileMode:    req.FileMode,
		Overwrite:   req.Overwrite,
		CreateDirs:  req.CreateDirs,
		Checksum:    checksum,
		MaxFileSize: 10 * 1024 * 1024 * 1024, // 10GB
	}

	// 10. 上传文件
	err = podClient.UploadFile(l.ctx, workloadInfo.Data.Namespace, req.PodName, containerName, destPath, file, uploadOpts)
	if err != nil {
		l.Errorf("上传文件失败: %v", err)
		return nil, fmt.Errorf("上传文件失败: %v", err)
	}

	// 11. 构建响应
	resp = &types.PodFileUploadResp{
		FilePath: destPath,
		FileSize: fileSize,
		Checksum: checksum,
	}

	l.Infof("成功上传文件: %s, 大小: %d 字节", destPath, fileSize)

	return resp, nil
}
