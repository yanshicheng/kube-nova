package cluster

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/client/storageservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateClusterAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateClusterAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateClusterAvatarLogic {
	return &UpdateClusterAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateClusterAvatarLogic) UpdateClusterAvatar(r *http.Request, req *types.ClusterUpdateAvatarRequest) (resp string, err error) {
	// 先获取集群详情用于审计日志
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return "", fmt.Errorf("获取集群详情失败: %w", err)
	}

	// 解析 multipart 表单
	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		l.Errorf("解析multipart表单失败: %v", err)
		return "", fmt.Errorf("解析表单失败: %w", err)
	}

	// 处理头像更新
	newUri := ""
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		file, handler, err := r.FormFile("avatar")
		if err == nil {
			defer func(file multipart.File) {
				err := file.Close()
				if err != nil {
					l.Errorf("关闭文件失败: %v", err)
				}
			}(file)
			l.Infof("接收到新头像文件: %s, 大小: %d bytes", handler.Filename, handler.Size)

			// 读取文件内容
			fileData, err := io.ReadAll(file)
			if err != nil {
				l.Errorf("读取头像文件失败: %v", err)
				return "", fmt.Errorf("读取文件失败")
			} else {
				image, err := l.svcCtx.StoreRpc.UploadImage(l.ctx, &storageservice.UploadImageRequest{
					ImageData: fileData,
					FileName:  handler.Filename,
					Project:   "cluster",
				})
				if err != nil {
					return "", err
				}
				newUri = image.ImageUri
			}
		}
	}

	_, err = l.svcCtx.ManagerRpc.ClusterUpdateIcon(l.ctx, &managerservice.ClusterUpdateAvatarReq{
		Id:     req.Id,
		Avatar: newUri,
	})
	if err != nil {
		l.Errorf("更新集群图标失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  clusterResp.Uuid,
			Title:        "更新集群图标",
			ActionDetail: fmt.Sprintf("更新集群图标失败，集群名称：%s", clusterResp.Name),
			Status:       0,
		})

		return "", err
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterResp.Uuid,
		Title:        "更新集群图标",
		ActionDetail: fmt.Sprintf("成功更新集群图标，集群名称：%s，新图标地址：%s", clusterResp.Name, newUri),
		Status:       1,
	})

	return "图标更新成功!", nil
}
