// internal/logic/pod/podfileuploadcompletelogic.go
package pod

import (
	"context"
	"fmt"
	"os"

	uploadcore2 "github.com/yanshicheng/kube-nova/application/console-api/internal/common/uploadcore"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	k8stypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodFileUploadCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPodFileUploadCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodFileUploadCompleteLogic {
	return &PodFileUploadCompleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodFileUploadCompleteLogic) PodFileUploadComplete(req *types.PodFileUploadCompleteReq) (resp *types.PodFileUploadCompleteResp, err error) {
	// 定义业务回调函数：实际上传到Pod
	callback := func(ctx context.Context, session *uploadcore2.UploadSession, tempFilePath string) (targetPath string, metadata map[string]string, err error) {
		// 提取元数据
		workloadId := uint64(session.Metadata["workloadId"].(float64))
		podName := session.Metadata["podName"].(string)
		destPath := session.Metadata["destPath"].(string)
		container := ""
		if session.Metadata["container"] != nil && session.Metadata["container"] != "" {
			container = session.Metadata["container"].(string)
		}
		fileMode := "0644"
		if session.Metadata["fileMode"] != nil && session.Metadata["fileMode"] != "" {
			fileMode = session.Metadata["fileMode"].(string)
		}

		// 获取工作空间信息
		workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: workloadId})
		if err != nil {
			return "", nil, fmt.Errorf("获取工作空间失败: %v", err)
		}

		// 获取K8s客户端
		client, err := l.svcCtx.K8sManager.GetCluster(ctx, workloadInfo.Data.ClusterUuid)
		if err != nil {
			return "", nil, fmt.Errorf("获取集群客户端失败: %v", err)
		}

		podClient := client.Pods()

		// 获取默认容器
		if container == "" {
			container, err = podClient.GetDefaultContainer(workloadInfo.Data.Namespace, podName)
			if err != nil {
				return "", nil, fmt.Errorf("获取默认容器失败: %v", err)
			}
		}

		// 打开临时文件
		tempFile, err := os.Open(tempFilePath)
		if err != nil {
			return "", nil, fmt.Errorf("打开临时文件失败: %v", err)
		}
		defer tempFile.Close()

		// 上传到Pod - 设置 10GB 限制
		uploadOpts := &k8stypes.UploadOptions{
			FileName:    session.FileName,
			FileSize:    session.FileSize,
			FileMode:    fileMode,
			Overwrite:   true,
			CreateDirs:  true,
			MaxFileSize: 10 * 1024 * 1024 * 1024, // 10GB 限制
		}

		l.Infof("开始上传文件到Pod: namespace=%s, pod=%s, container=%s, size=%d",
			workloadInfo.Data.Namespace, podName, container, session.FileSize)

		err = podClient.UploadFile(ctx, workloadInfo.Data.Namespace, podName, container, destPath, tempFile, uploadOpts)
		if err != nil {
			return "", nil, fmt.Errorf("上传到Pod失败: %v", err)
		}

		// 返回结果
		return destPath, map[string]string{
			"namespace": workloadInfo.Data.Namespace,
			"pod":       podName,
			"container": container,
		}, nil
	}

	// 调用通用上传服务完成上传
	uploadSvc := uploadcore2.GetService()
	result, err := uploadSvc.Complete(l.ctx, req.SessionId, callback)
	if err != nil {
		l.Errorf("完成上传失败: %v", err)
		return nil, err
	}

	// 验证校验和
	if req.FinalChecksum != "" && result.Checksum != req.FinalChecksum {
		return nil, fmt.Errorf("文件校验和不匹配")
	}

	resp = &types.PodFileUploadCompleteResp{
		FilePath:   result.TargetPath,
		FileSize:   result.FileSize,
		Checksum:   result.Checksum,
		UploadTime: result.UploadTime,
	}

	l.Infof("完成Pod文件上传: path=%s, size=%d", result.TargetPath, result.FileSize)
	return resp, nil
}
