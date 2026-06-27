package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/tektonsync"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonStepImageBatchUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonStepImageBatchUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonStepImageBatchUpdateLogic {
	return &TektonStepImageBatchUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonStepImageBatchUpdateLogic) TektonStepImageBatchUpdate(in *pb.BatchUpdateTektonStepImageReq) (*pb.BatchUpdateTektonStepImageResp, error) {
	if len(in.Items) == 0 {
		return nil, errorx.Msg("请选择要更新的镜像")
	}
	grouped := make(map[string][]tektonStepImageReplacement, len(in.Items))
	ids := make([]string, 0, len(in.Items))
	seen := make(map[string]struct{}, len(in.Items))
	for _, item := range in.Items {
		if item == nil || item.Id == "" {
			return nil, errorx.Msg("步骤 ID 不能为空")
		}
		replacement, err := tektonStepImageReplacementFromPb(item)
		if err != nil {
			return nil, err
		}
		grouped[item.Id] = append(grouped[item.Id], replacement)
		if _, ok := seen[item.Id]; !ok {
			ids = append(ids, item.Id)
			seen[item.Id] = struct{}{}
		}
	}
	syncer := tektonsync.NewSyncer(l.svcCtx)
	updated := uint64(0)
	for _, id := range ids {
		exist, err := l.svcCtx.StepTemplateModel.FindOne(l.ctx, id)
		if err != nil {
			l.Errorf("Tekton步骤镜像更新失败: %v", err)
			return nil, err
		}
		if exist.EngineType != tektonEngineType {
			l.Errorf("只能更新 Tekton 步骤镜像")
			return nil, errorx.Msg("只能更新 Tekton 步骤镜像")
		}
		stageContent, changed, err := replaceTektonStepImages(exist.StageContent, grouped[id])
		if err != nil {
			l.Errorf("Tekton步骤镜像更新失败: %v", err)
			return nil, err
		}
		if !changed {
			continue
		}
		data, err := buildTektonStepImageUpdateData(l.ctx, l.svcCtx, exist, stageContent, in.UpdatedBy)
		if err != nil {
			l.Errorf("Tekton步骤镜像更新失败: %v", err)
			return nil, err
		}
		if data.Status == 1 {
			if err := syncer.SyncStepToAllChannels(l.ctx, data, in.UpdatedBy); err != nil {
				l.Errorf("Tekton步骤镜像同步失败: %v", err)
				if exist.Status == 1 {
					if restoreErr := syncer.SyncStepToAllChannels(l.ctx, exist, in.UpdatedBy); restoreErr != nil {
						l.Errorf("Tekton步骤镜像同步失败后恢复旧资源失败: %v", restoreErr)
					}
				}
				return nil, errorx.Msg("Tekton 步骤同步失败：" + devopstypes.TrimMessage(err.Error()))
			}
		}
		if err := l.svcCtx.StepTemplateModel.Update(l.ctx, data); err != nil {
			l.Errorf("Tekton步骤镜像更新失败: %v", err)
			if data.Status == 1 && exist.Status == 1 {
				if restoreErr := syncer.SyncStepToAllChannels(l.ctx, exist, in.UpdatedBy); restoreErr != nil {
					l.Errorf("Tekton步骤镜像数据库更新失败后恢复旧资源失败: %v", restoreErr)
				}
			}
			return nil, err
		}
		updated++
	}

	return &pb.BatchUpdateTektonStepImageResp{Total: uint64(len(in.Items)), Updated: updated}, nil
}
