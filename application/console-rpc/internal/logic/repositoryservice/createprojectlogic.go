package repositoryservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProjectLogic {
	return &CreateProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// convertStorageToBytes 将存储大小和单位转换为字节数
func convertStorageToBytes(size int64, unit string) (int64, error) {
	// -1 或 0 表示无限制
	if size <= 0 {
		return -1, nil
	}

	// 如果没有指定单位，默认为 GB
	if unit == "" {
		unit = "GB"
	}

	var multiplier int64
	switch unit {
	case "B", "b":
		multiplier = 1
	case "KB", "kb":
		multiplier = 1024
	case "MB", "mb":
		multiplier = 1024 * 1024
	case "GB", "gb":
		multiplier = 1024 * 1024 * 1024
	case "TB", "tb":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("不支持的存储单位: %s，支持的单位: B, KB, MB, GB, TB", unit)
	}

	return size * multiplier, nil
}

func (l *CreateProjectLogic) CreateProject(in *pb.CreateProjectReq) (*pb.CreateProjectResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 🔧 修复：将 int64 + storageUnit 转换为字节数
	var storageLimit int64 = -1 // 默认 -1 表示无限制
	if in.StorageLimit != 0 {
		storageLimit, err = convertStorageToBytes(in.StorageLimit, in.StorageUnit)
		if err != nil {
			return nil, errorx.Msg("存储大小格式错误: " + err.Error())
		}
	}

	req := &types.ProjectReq{
		ProjectName:  in.ProjectName,
		Public:       in.IsPublic,
		StorageLimit: storageLimit, // 使用转换后的字节数
	}

	err = client.Project().Create(req)
	if err != nil {
		return nil, errorx.Msg("创建项目失败")
	}

	// 如果指定了应用项目和集群，自动绑定
	if in.AppProjectId != 0 && in.ClusterUuid != "" {
		// 查找 registry_cluster_id
		clusters, _ := l.svcCtx.RepositoryClusterModel.SearchNoPage(
			l.ctx,
			"",
			true,
			"`cluster_uuid` = ?",
			in.ClusterUuid,
		)

		for _, c := range clusters {
			reg, _ := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, c.RegistryId)
			if reg != nil && reg.Uuid == in.RegistryUuid {
				// 创建绑定
				binding := &repository.RegistryProjectBinding{
					RegistryId:          reg.Id, // 仓库id
					AppProjectId:        in.AppProjectId,
					RegistryProjectName: in.ProjectName,
					IsDeleted:           0,
				}
				_, err := l.svcCtx.RegistryProjectBindingModel.Insert(l.ctx, binding)
				if err != nil {
					l.Errorf("创建绑定失败: %v", err)
					return nil, errorx.Msg("创建项目绑定失败")
				}
				break
			}
		}
	}

	// 获取创建的项目信息
	project, _ := client.Project().Get(in.ProjectName)
	projectId := int64(0)
	if project != nil {
		projectId = project.ProjectID
	}

	return &pb.CreateProjectResp{
		ProjectId: projectId,
		Message:   "创建成功",
	}, nil
}
