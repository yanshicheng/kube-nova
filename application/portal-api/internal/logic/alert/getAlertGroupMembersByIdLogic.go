package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertGroupMembersByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAlertGroupMembersByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertGroupMembersByIdLogic {
	return &GetAlertGroupMembersByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertGroupMembersByIdLogic) GetAlertGroupMembersById(req *types.DefaultIdRequest) (resp *types.AlertGroupMembers, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("获取告警组成员请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务获取告警组成员
	result, err := l.svcCtx.AlertPortalRpc.AlertGroupMembersGetById(l.ctx, &pb.GetAlertGroupMembersByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取告警组成员失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return nil, err
	}

	// 转换数据，包含用户名和账号信息
	resp = &types.AlertGroupMembers{
		Id:          result.Data.Id,
		GroupId:     result.Data.GroupId,
		UserId:      result.Data.UserId,
		UserName:    result.Data.UserName,
		UserAccount: result.Data.UserAccount,
		Role:        result.Data.Role,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	l.Infof("获取告警组成员成功: operator=%s, id=%d", username, req.Id)
	return resp, nil
}
