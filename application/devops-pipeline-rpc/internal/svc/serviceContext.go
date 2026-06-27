package svc

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/config"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-quality-rpc/client/qualityservice"
	"github.com/yanshicheng/kube-nova/common/interceptors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config            config.Config
	Cache             *redis.Redis
	PipelineModel     *model.DevopsPipelineModel
	PipelineRunModel  *model.DevopsPipelineRunModel
	RunCounterModel   *model.DevopsPipelineRunCounterModel
	RunStageModel     *model.DevopsPipelineRunStageModel
	ArtifactModel     *model.DevopsPipelineRunArtifactModel
	MetricModel       *model.DevopsPipelineMetricModel
	ProjectRpc        projectservice.ProjectService
	PipelineConfigRpc pipelineconfigservice.PipelineConfigService
	QualityRpc        qualityservice.QualityService
}

func NewServiceContext(c config.Config) *ServiceContext {
	managerRpc := zrpc.MustNewClient(c.DevopsManagerRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	qualityRpc := zrpc.MustNewClient(c.DevopsQualityRpc,
		zrpc.WithUnaryClientInterceptor(interceptors.ClientMetadataInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientErrorInterceptor()),
	)
	ctx := &ServiceContext{
		Config:            c,
		Cache:             redis.MustNewRedis(c.Cache),
		PipelineModel:     model.NewDevopsPipelineModel(c.Mongo.Url, c.Mongo.Db),
		PipelineRunModel:  model.NewDevopsPipelineRunModel(c.Mongo.Url, c.Mongo.Db),
		RunCounterModel:   model.NewDevopsPipelineRunCounterModel(c.Mongo.Url, c.Mongo.Db),
		RunStageModel:     model.NewDevopsPipelineRunStageModel(c.Mongo.Url, c.Mongo.Db),
		ArtifactModel:     model.NewDevopsPipelineRunArtifactModel(c.Mongo.Url, c.Mongo.Db),
		MetricModel:       model.NewDevopsPipelineMetricModel(c.Mongo.Url, c.Mongo.Db),
		ProjectRpc:        projectservice.NewProjectService(managerRpc),
		PipelineConfigRpc: pipelineconfigservice.NewPipelineConfigService(managerRpc),
		QualityRpc:        qualityservice.NewQualityService(qualityRpc),
	}
	ctx.ensureIndexesAsync()
	return ctx
}

func (s *ServiceContext) ensureIndexesAsync() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.PipelineModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化流水线索引失败: %v", err)
		}
		if err := s.PipelineRunModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化流水线运行索引失败: %v", err)
		}
		if err := s.RunCounterModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化流水线运行计数器索引失败: %v", err)
		}
		if err := s.RunStageModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化流水线阶段索引失败: %v", err)
		}
		if err := s.ArtifactModel.EnsureIndexes(ctx); err != nil {
			logx.Errorf("初始化流水线产物索引失败: %v", err)
		}
	}()
}
