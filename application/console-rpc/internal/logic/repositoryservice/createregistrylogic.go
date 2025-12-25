package repositoryservicelogic

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRegistryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRegistryLogic {
	return &CreateRegistryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 镜像仓库管理（管理员操作）============
func (l *CreateRegistryLogic) CreateRegistry(in *pb.CreateRegistryReq) (*pb.CreateRegistryResp, error) {
	// 生成 UUID
	registryUUID := uuid.New().String()

	// 准备数据
	data := &repository.ContainerRegistry{
		Name:        in.Name,
		Uuid:        registryUUID,
		Type:        in.Type,
		Env:         in.Env,
		Url:         in.Url,
		Username:    in.Username,
		Password:    in.Password,
		Insecure:    int64(0),
		Status:      int64(in.Status),
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
		IsDeleted:   0,
	}

	if in.Insecure {
		data.Insecure = 1
	}

	// 处理可选字段
	if in.CaCert != "" {
		data.CaCert = sql.NullString{String: in.CaCert, Valid: true}
	}
	if in.Config != "" {
		data.Config = sql.NullString{String: in.Config, Valid: true}
	}

	// 插入数据库
	_, err := l.svcCtx.ContainerRegistryModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("创建镜像仓库失败")
	}

	return &pb.CreateRegistryResp{
		Uuid:    registryUUID,
		Message: "创建成功",
	}, nil
}
