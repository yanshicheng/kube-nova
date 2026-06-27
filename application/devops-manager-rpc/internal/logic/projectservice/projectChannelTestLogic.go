package projectservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/channelcheck"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelTestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelTestLogic {
	return &ProjectChannelTestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelTestLogic) ProjectChannelTest(in *pb.GetByIdReq) (*pb.TestChannelResp, error) {
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目渠道测试失败: %v", err)
		return nil, err
	}
	if err := l.ensureProjectAccess(binding.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("项目渠道测试失败: %v", err)
		return nil, err
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, binding.ChannelID)
	if err != nil {
		l.Errorf("项目渠道测试失败: %v", err)
		return nil, err
	}

	credentialID, err := l.resolveCredentialID(binding, channel)
	if err != nil {
		l.Errorf("项目渠道测试失败: %v", err)
		return nil, err
	}
	if credentialID == "" {
		message := "未配置项目级凭据，且当前绑定不允许使用渠道全局凭据"
		if binding.AllowUseGlobalCredential {
			message = "渠道全局凭据未配置"
		}
		return l.saveAndReturn(binding.ID.Hex(), unhealthyProjectChannel(message), "system")
	}

	testChannel := *channel
	testChannel.CredentialID = credentialID
	result := l.svcCtx.ChannelChecker.Test(l.ctx, &testChannel)
	return l.saveAndReturn(binding.ID.Hex(), result, "system")
}

func (l *ProjectChannelTestLogic) ensureProjectAccess(projectID string, userID uint64, roles []string) error {
	if isSuperAdminRole(roles) {
		return nil
	}
	projectIDs, _, err := memberProjectScope(l.ctx, l.svcCtx, userID, roles)
	if err != nil {
		l.Errorf("校验项目权限失败: %v", err)
		return err
	}
	for _, item := range projectIDs {
		if item == projectID {
			return nil
		}
	}
	l.Errorf("无权操作该项目渠道")
	return errorx.Msg("无权操作该项目渠道")
}

func (l *ProjectChannelTestLogic) resolveCredentialID(binding *model.DevopsProjectChannelBinding, channel *model.DevopsChannel) (string, error) {
	projectCredentialID := strings.TrimSpace(binding.ProjectCredentialID)
	if projectCredentialID != "" {
		if err := validateProjectBindingCredential(l.ctx, l.svcCtx, binding.ProjectID, resolveBindingGroupCode(l.ctx, l.svcCtx, binding), channel.ChannelType, true, projectCredentialID); err != nil {
			l.Errorf("解析凭证 ID 失败: %v", err)
			return "", err
		}
		return projectCredentialID, nil
	}
	if binding.AllowUseGlobalCredential {
		return channel.CredentialID, nil
	}
	return "", nil
}

func (l *ProjectChannelTestLogic) saveAndReturn(bindingID string, result channelcheck.Result, operator string) (*pb.TestChannelResp, error) {
	if err := l.svcCtx.ProjectChannelModel.UpdateHealth(l.ctx, bindingID, result.HealthStatus, result.Message, result.Metadata, operator, result.CheckedAt); err != nil {
		l.Errorf("项目渠道连通性状态更新失败: %v", err)
	}
	return &pb.TestChannelResp{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}

func unhealthyProjectChannel(message string) channelcheck.Result {
	return channelcheck.Result{
		Success:      false,
		HealthStatus: "unhealthy",
		Message:      message,
		CheckedAt:    time.Now().Unix(),
	}
}
