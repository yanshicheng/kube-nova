package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目成员详情
func NewGetProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectMemberLogic {
	return &GetProjectMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectMemberLogic) GetProjectMember(req *types.GetProjectMemberRequest) (resp *types.GetProjectMemberResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetProjectMember(l.ctx, &pb.GetProjectMemberReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		MemberId:     req.MemberId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取项目成员详情成功: MemberId=%d, EntityName=%s", data.Id, data.EntityName)

	return &types.GetProjectMemberResponse{
		Data: types.ProjectMember{
			Id:           data.Id,
			ProjectId:    data.ProjectId,
			EntityName:   data.EntityName,
			EntityType:   data.EntityType,
			RoleId:       data.RoleId,
			RoleName:     data.RoleName,
			CreationTime: data.CreationTime,
			UpdateTime:   data.UpdateTime,
		},
	}, nil
}
