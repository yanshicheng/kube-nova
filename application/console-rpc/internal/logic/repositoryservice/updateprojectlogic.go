package repositoryservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectLogic {
	return &UpdateProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// convertStorageToBytes 将存储大小和单位转换为字节数
func (l *UpdateProjectLogic) convertStorageToBytes(size int64, unit string) (int64, error) {
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

func (l *UpdateProjectLogic) UpdateProject(in *pb.UpdateProjectReq) (*pb.UpdateProjectResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 🔧 修复：将 int64 + storageUnit 转换为字节数
	var storageLimit int64 = -1 // 默认 -1 表示无限制
	if in.StorageLimit != 0 {
		storageLimit, err = l.convertStorageToBytes(in.StorageLimit, in.StorageUnit)
		if err != nil {
			return nil, errorx.Msg("存储大小格式错误: " + err.Error())
		}
	}

	req := &types.ProjectReq{
		ProjectName:  in.ProjectName,
		Public:       in.IsPublic,
		StorageLimit: storageLimit, // 使用转换后的字节数
	}

	err = client.Project().Update(in.ProjectName, req)
	if err != nil {
		return nil, errorx.Msg("更新项目失败")
	}

	return &pb.UpdateProjectResp{Message: "更新成功"}, nil
}
