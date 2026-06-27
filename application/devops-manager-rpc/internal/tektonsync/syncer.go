package tektonsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	devopstekton "github.com/yanshicheng/kube-nova/common/devops/tekton"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	EngineTypeTekton = "tekton"
	SyncStatusSynced = "synced"
	SyncStatusFailed = "failed"
)

type BindingConfig struct {
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"serviceAccountName"`
	PipelinePrefix     string `json:"pipelinePrefix"`
}

type Syncer struct {
	svcCtx *svc.ServiceContext
}

func NewSyncer(svcCtx *svc.ServiceContext) *Syncer {
	return &Syncer{svcCtx: svcCtx}
}

func ProjectTektonNamespace(projectCode string) string {
	return devopstekton.ProjectNamespace(projectCode)
}

func NormalizeBindingConfigForProject(projectCode, content string) (string, error) {
	var cfg BindingConfig
	content = strings.TrimSpace(content)
	if content != "" {
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			return "", errorx.Msg("Tekton 渠道绑定配置必须是结构化 JSON")
		}
	}
	cfg.Namespace = ProjectTektonNamespace(projectCode)
	cfg.ServiceAccountName = strings.TrimSpace(cfg.ServiceAccountName)
	cfg.PipelinePrefix = strings.TrimSpace(cfg.PipelinePrefix)
	payload, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (s *Syncer) SyncStepToAllChannels(ctx context.Context, step *model.DevopsStepTemplate, operator string) error {
	if step == nil || step.EngineType != EngineTypeTekton || step.Status != 1 {
		return nil
	}
	channels, err := s.activeTektonChannels(ctx)
	if err != nil {
		return err
	}
	var failed []string
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if err := s.SyncStepToChannel(ctx, step, channel, operator); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", channel.Name, devopstypes.TrimMessage(err.Error())))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 Tekton 实例同步失败: %s", strings.Join(failed, "、"))
	}
	return nil
}

func (s *Syncer) SyncAllStepsToAllChannels(ctx context.Context, operator string) error {
	channels, err := s.activeTektonChannels(ctx)
	if err != nil {
		return err
	}
	var failed []string
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if err := s.SyncAllStepsToChannel(ctx, channel, operator); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", channel.Name, devopstypes.TrimMessage(err.Error())))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 Tekton 实例同步失败: %s", strings.Join(failed, "、"))
	}
	return nil
}

func (s *Syncer) SyncAllStepsToChannel(ctx context.Context, channel *model.DevopsChannel, operator string) error {
	if channel == nil || channel.ChannelType != EngineTypeTekton || channel.Status != 1 {
		return nil
	}
	var failed []string
	page := uint64(1)
	for {
		steps, total, err := s.svcCtx.StepTemplateModel.List(ctx, model.DevopsStepTemplateListFilter{
			EngineType: EngineTypeTekton,
			Status:     1,
			Page:       page,
			PageSize:   200,
		})
		if err != nil {
			logx.Errorf("查询 Tekton 步骤库失败: %v", err)
			return err
		}
		for _, step := range steps {
			if err := s.SyncStepToChannel(ctx, step, channel, operator); err != nil {
				failed = append(failed, step.Name)
			}
		}
		if uint64(len(steps)) == 0 || page*200 >= total {
			break
		}
		page++
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 Tekton 步骤同步失败: %s", strings.Join(failed, "、"))
	}
	return nil
}

func (s *Syncer) DeleteAllStepsFromChannel(ctx context.Context, channel *model.DevopsChannel, operator string) error {
	if channel == nil || channel.ChannelType != EngineTypeTekton {
		return nil
	}
	var failed []string
	page := uint64(1)
	for {
		steps, total, err := s.svcCtx.StepTemplateModel.List(ctx, model.DevopsStepTemplateListFilter{
			EngineType: EngineTypeTekton,
			Status:     -1,
			Page:       page,
			PageSize:   200,
		})
		if err != nil {
			logx.Errorf("查询 Tekton 步骤库失败: %v", err)
			return err
		}
		for _, step := range steps {
			if err := s.DeleteStepFromChannel(ctx, step, channel, operator); err != nil {
				failed = append(failed, step.Name)
			}
		}
		if uint64(len(steps)) == 0 || page*200 >= total {
			break
		}
		page++
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 Tekton 步骤删除失败: %s", strings.Join(failed, "、"))
	}
	return nil
}

func (s *Syncer) SyncStepToChannel(ctx context.Context, step *model.DevopsStepTemplate, channel *model.DevopsChannel, operator string) error {
	resource, parseErr := devopstekton.ParseStepResource(step.StageContent)
	record := syncRecord(step, channel, resource, operator)
	if parseErr != nil {
		record.SyncStatus = SyncStatusFailed
		record.SyncMessage = devopstypes.TrimMessage(parseErr.Error())
		if err := s.svcCtx.StepSyncModel.Upsert(ctx, record); err != nil {
			logx.Errorf("记录 Tekton 步骤同步失败状态失败: step=%s channel=%s err=%v", step.Code, channel.Code, err)
		}
		return parseErr
	}
	cfg, err := devopstekton.ParseChannelConfig(channel.Config)
	if err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	client, err := s.clientFromChannel(ctx, channel)
	if err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	if err := client.NamespaceExists(ctx, cfg.TaskNamespace); err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	resource, err = client.ApplyStepResource(ctx, cfg.TaskNamespace, step.StageContent)
	record = syncRecord(step, channel, resource, operator)
	if err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	record.SyncStatus = SyncStatusSynced
	record.SyncMessage = ""
	record.LastSyncAt = time.Now()
	if err := s.svcCtx.StepSyncModel.Upsert(ctx, record); err != nil {
		logx.Errorf("记录 Tekton 步骤同步状态失败: step=%s channel=%s err=%v", step.Code, channel.Code, err)
		return err
	}
	return nil
}

func (s *Syncer) DeleteStepFromAllChannels(ctx context.Context, step *model.DevopsStepTemplate, operator string) error {
	if step == nil || step.EngineType != EngineTypeTekton {
		return nil
	}
	channels, err := s.allTektonChannels(ctx)
	if err != nil {
		return err
	}
	var failed []string
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if err := s.DeleteStepFromChannel(ctx, step, channel, operator); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", channel.Name, devopstypes.TrimMessage(err.Error())))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 Tekton 实例删除步骤失败: %s", strings.Join(failed, "、"))
	}
	return nil
}

func (s *Syncer) DeleteStepFromChannel(ctx context.Context, step *model.DevopsStepTemplate, channel *model.DevopsChannel, operator string) error {
	resource, parseErr := devopstekton.ParseStepResource(step.StageContent)
	record := syncRecord(step, channel, resource, operator)
	if parseErr != nil {
		return s.saveSyncFailed(ctx, record, parseErr)
	}
	cfg, err := devopstekton.ParseChannelConfig(channel.Config)
	if err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	client, err := s.clientFromChannel(ctx, channel)
	if err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	if _, err := client.DeleteStepResource(ctx, cfg.TaskNamespace, step.StageContent); err != nil {
		return s.saveSyncFailed(ctx, record, err)
	}
	resource.Namespace = cfg.TaskNamespace
	record.SyncStatus = "deleted"
	record.SyncMessage = ""
	record.LastSyncAt = time.Now()
	if err := s.svcCtx.StepSyncModel.Upsert(ctx, record); err != nil {
		logx.Errorf("记录 Tekton 步骤删除同步状态失败: step=%s channel=%s err=%v", step.Code, channel.Code, err)
		return err
	}
	return nil
}

func ValidateChannelConfig(ctx context.Context, svcCtx *svc.ServiceContext, channel *model.DevopsChannel) error {
	if channel == nil || channel.ChannelType != EngineTypeTekton {
		return nil
	}
	cfg, err := devopstekton.ParseChannelConfig(channel.Config)
	if err != nil {
		logx.Errorf("Tekton 渠道配置错误: %v", err)
		return errorx.Msg(err.Error())
	}
	client, err := NewSyncer(svcCtx).clientFromChannel(ctx, channel)
	if err != nil {
		logx.Errorf("Tekton 渠道配置校验失败: %v", err)
		return err
	}
	if err := client.CheckTektonAPI(ctx); err != nil {
		logx.Errorf("Tekton API 校验失败: %v", err)
		return errorx.Msg(err.Error())
	}
	if err := client.CheckClusterResolver(ctx); err != nil {
		logx.Errorf("Tekton cluster resolver 校验失败: %v", err)
		return errorx.Msg(err.Error())
	}
	if err := client.NamespaceExists(ctx, cfg.TaskNamespace); err != nil {
		logx.Errorf("Tekton taskNamespace 校验失败: %v", err)
		return errorx.Msg(err.Error())
	}
	return nil
}

func ValidateBindingConfig(ctx context.Context, svcCtx *svc.ServiceContext, channel *model.DevopsChannel, bindingConfig string) error {
	if channel == nil || channel.ChannelType != EngineTypeTekton {
		return nil
	}
	cfg, err := ParseBindingConfig(bindingConfig)
	if err != nil {
		logx.Errorf("Tekton 项目渠道配置错误: %v", err)
		return err
	}
	client, err := NewSyncer(svcCtx).clientFromChannel(ctx, channel)
	if err != nil {
		logx.Errorf("Tekton 项目渠道配置校验失败: %v", err)
		return err
	}
	if err := client.EnsureNamespace(ctx, cfg.Namespace); err != nil {
		logx.Errorf("Tekton namespace 创建或校验失败: %v", err)
		return errorx.Msg(err.Error())
	}
	if name := strings.TrimSpace(cfg.ServiceAccountName); name != "" && name != "default" {
		if err := client.ServiceAccountExists(ctx, cfg.Namespace, name); err != nil {
			logx.Errorf("Tekton serviceAccount 校验失败: %v", err)
			return errorx.Msg(err.Error())
		}
	}
	return nil
}

func ParseBindingConfig(content string) (BindingConfig, error) {
	var cfg BindingConfig
	content = strings.TrimSpace(content)
	if content == "" {
		return cfg, errorx.Msg("Tekton namespace 不能为空")
	}
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return cfg, errorx.Msg("Tekton 渠道绑定配置必须是结构化 JSON")
	}
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)
	cfg.ServiceAccountName = strings.TrimSpace(cfg.ServiceAccountName)
	cfg.PipelinePrefix = strings.TrimSpace(cfg.PipelinePrefix)
	if cfg.Namespace == "" {
		return cfg, errorx.Msg("Tekton namespace 不能为空")
	}
	return cfg, nil
}

func (s *Syncer) ClientFromChannel(ctx context.Context, channel *model.DevopsChannel) (*devopstekton.Client, error) {
	return s.clientFromChannel(ctx, channel)
}

func (s *Syncer) clientFromChannel(ctx context.Context, channel *model.DevopsChannel) (*devopstekton.Client, error) {
	req, err := s.requestFromChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	client, err := devopstekton.NewClient(req)
	if err != nil {
		logx.Errorf("创建 Tekton 客户端失败: channel=%s err=%v", channel.Code, err)
		return nil, err
	}
	return client, nil
}

func (s *Syncer) requestFromChannel(ctx context.Context, channel *model.DevopsChannel) (devopstypes.Request, error) {
	req := devopstypes.Request{
		Channel: devopstypes.Channel{
			ID:              channel.ID.Hex(),
			Name:            channel.Name,
			Code:            channel.Code,
			Type:            channel.ChannelType,
			Endpoint:        channel.Endpoint,
			Config:          channel.Config,
			AuthType:        channel.AuthType,
			Username:        channel.Username,
			Password:        channel.Password,
			Token:           channel.Token,
			InsecureSkipTLS: channel.InsecureSkipTLS,
		},
	}
	credentialID := strings.TrimSpace(channel.CredentialID)
	if credentialID == "" {
		credentialID = strings.TrimSpace(channel.GlobalCredentialID)
	}
	if credentialID == "" {
		return req, nil
	}
	credential, err := s.svcCtx.CredentialModel.FindOne(ctx, credentialID)
	if err != nil {
		logx.Errorf("读取 Tekton 渠道凭据失败: channel=%s credentialId=%s err=%v", channel.Code, credentialID, err)
		return req, err
	}
	if credential.Status != 1 {
		return req, errorx.Msg("Tekton 渠道凭据已停用")
	}
	secret, err := credential.Secret()
	if err != nil {
		logx.Errorf("解析 Tekton 渠道凭据失败: channel=%s credentialId=%s err=%v", channel.Code, credentialID, err)
		return req, err
	}
	req.Credential = &devopstypes.Credential{
		Type:        credential.CredentialType,
		Username:    secret.Username,
		Password:    secret.Password,
		Token:       secret.Token,
		PrivateKey:  secret.PrivateKey,
		Passphrase:  secret.Passphrase,
		Kubeconfig:  secret.Kubeconfig,
		SecretText:  secret.SecretText,
		Certificate: secret.Certificate,
		JsonData:    secret.JsonData,
	}
	return req, nil
}

func syncRecord(step *model.DevopsStepTemplate, channel *model.DevopsChannel, resource devopstekton.StepResource, operator string) *model.DevopsStepSync {
	return &model.DevopsStepSync{
		StepID:             step.ID.Hex(),
		StepCode:           step.Code,
		StepName:           step.Name,
		ChannelID:          channel.ID.Hex(),
		ChannelCode:        channel.Code,
		ChannelName:        channel.Name,
		ResourceKind:       resource.Kind,
		ResourceName:       resource.Name,
		ResourceNamespace:  resource.Namespace,
		ResourceAPIVersion: resource.APIVersion,
		ResourceHash:       resource.Hash,
		CreatedBy:          operator,
		UpdatedBy:          operator,
		LastSyncAt:         time.Now(),
	}
}

func (s *Syncer) activeTektonChannels(ctx context.Context) ([]*model.DevopsChannel, error) {
	return s.tektonChannels(ctx, 1)
}

func (s *Syncer) allTektonChannels(ctx context.Context) ([]*model.DevopsChannel, error) {
	return s.tektonChannels(ctx, -1)
}

func (s *Syncer) tektonChannels(ctx context.Context, status int64) ([]*model.DevopsChannel, error) {
	channels, err := s.svcCtx.ChannelModel.ListAll(ctx)
	if err != nil {
		logx.Errorf("查询 Tekton 渠道实例失败: %v", err)
		return nil, err
	}
	result := make([]*model.DevopsChannel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.ChannelType != EngineTypeTekton {
			continue
		}
		if status >= 0 && channel.Status != status {
			continue
		}
		result = append(result, channel)
	}
	return result, nil
}

func (s *Syncer) saveSyncFailed(ctx context.Context, record *model.DevopsStepSync, err error) error {
	record.SyncStatus = SyncStatusFailed
	record.SyncMessage = devopstypes.TrimMessage(err.Error())
	record.LastSyncAt = time.Now()
	if upsertErr := s.svcCtx.StepSyncModel.Upsert(ctx, record); upsertErr != nil {
		logx.Errorf("记录 Tekton 步骤同步失败状态失败: step=%s channel=%s err=%v", record.StepCode, record.ChannelCode, upsertErr)
		return upsertErr
	}
	logx.Errorf("Tekton 步骤同步失败: step=%s channel=%s err=%v", record.StepCode, record.ChannelCode, err)
	return err
}
