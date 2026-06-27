package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepImageListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepImageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepImageListLogic {
	return &TektonStepImageListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepImageListLogic) TektonStepImageList(in *pb.ListTektonStepImageReq) (*pb.ListTektonStepImageResp, error) {
	steps, err := l.svcCtx.StepTemplateModel.ListForImageManage(l.ctx, model.DevopsStepTemplateListFilter{
		Name:       in.Name,
		Code:       in.Code,
		CategoryID: in.CategoryId,
		EngineType: tektonEngineType,
		Type:       stepTypeTask,
		Status:     in.Status,
	})
	if err != nil {
		l.Errorf("Tekton步骤镜像查询失败: %v", err)
		return nil, err
	}
	allItems := make([]tektonStepImageItem, 0, len(steps))
	for _, step := range steps {
		items, err := collectTektonStepImagesFromTemplate(step)
		if err != nil {
			l.Errorf("解析 Tekton 步骤镜像失败: step=%s err=%v", step.Code, err)
			continue
		}
		allItems = append(allItems, items...)
	}
	total := uint64(len(allItems))
	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 50
	}
	start := (page - 1) * pageSize
	if start >= total {
		return &pb.ListTektonStepImageResp{Data: []*pb.TektonStepImageItem{}, Total: total}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]*pb.TektonStepImageItem, 0, end-start)
	for _, item := range allItems[start:end] {
		items = append(items, tektonStepImageItemToPb(item))
	}

	return &pb.ListTektonStepImageResp{Data: items, Total: total}, nil
}
