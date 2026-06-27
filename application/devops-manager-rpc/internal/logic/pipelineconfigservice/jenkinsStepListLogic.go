package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JenkinsStepListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJenkinsStepListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JenkinsStepListLogic {
	return &JenkinsStepListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JenkinsStepListLogic) JenkinsStepList(in *pb.ListStepTemplateReq) (*pb.ListStepTemplateResp, error) {
	engineType := in.EngineType
	if engineType == "" {
		engineType = jenkinsEngineType
	}
	data, total, err := l.svcCtx.StepTemplateModel.List(l.ctx, model.DevopsStepTemplateListFilter{
		Page:       in.Page,
		PageSize:   in.PageSize,
		Name:       in.Name,
		Code:       in.Code,
		CategoryID: in.CategoryId,
		EngineType: engineType,
		Type:       in.Type,
		Status:     in.Status,
	})
	if err != nil {
		l.Errorf("Jenkins步骤查询列表失败: %v", err)
		return nil, err
	}
	categoryIDs := make([]string, 0, len(data))
	for _, item := range data {
		if item.CategoryID != "" {
			categoryIDs = append(categoryIDs, item.CategoryID)
		}
	}
	categories, err := l.svcCtx.StepCategoryModel.FindByIDs(l.ctx, categoryIDs)
	if err != nil {
		l.Errorf("Jenkins步骤查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsStepTemplate, 0, len(data))
	for _, item := range data {
		categoryName := ""
		categoryCode := ""
		if category := categories[item.CategoryID]; category != nil {
			categoryName = category.Name
			categoryCode = category.Code
		}
		items = append(items, stepTemplateToPbWithCategory(item, categoryName, categoryCode))
	}

	return &pb.ListStepTemplateResp{Data: items, Total: total}, nil
}
