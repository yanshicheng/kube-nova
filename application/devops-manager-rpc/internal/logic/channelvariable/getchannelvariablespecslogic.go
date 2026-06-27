package logic

import (
	"context"
	"encoding/json"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetChannelVariableSpecsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetChannelVariableSpecsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChannelVariableSpecsLogic {
	return &GetChannelVariableSpecsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetChannelVariableSpecs 获取渠道变量规格
func (l *GetChannelVariableSpecsLogic) GetChannelVariableSpecs(in *pb.GetChannelVariableSpecsReq) (*pb.GetChannelVariableSpecsResp, error) {
	// 从数据库加载规格
	var specs []*model.ChannelVariableSpec
	var err error

	if in.SpecProfile != "" {
		// 按规格模板查询
		specs, err = l.svcCtx.ChannelVariableSpecModel.FindBySpecProfile(l.ctx, in.SpecProfile)
	} else if in.ChannelTypeCode != "" {
		// 按渠道类型查询
		// TODO: DevopsChannelType需要添加SpecProfile字段
		// 暂时使用映射关系
		specProfile := getSpecProfileByChannelType(in.ChannelTypeCode)
		if specProfile != "" {
			specs, err = l.svcCtx.ChannelVariableSpecModel.FindBySpecProfile(l.ctx, specProfile)
		}
	} else if in.GroupCode != "" {
		// 按分组查询（返回该分组下所有渠道类型的规格）
		specs, err = l.svcCtx.ChannelVariableSpecModel.FindByGroupCode(l.ctx, in.GroupCode)
	} else {
		// 查询所有规格
		specs, err = l.svcCtx.ChannelVariableSpecModel.FindAll(l.ctx)
	}

	if err != nil {
		l.Errorf("查询渠道变量规格失败: %v", err)
		return nil, err
	}

	// 转换为pb格式
	pbSpecs := make([]*pb.VariableSpec, len(specs))
	for i, spec := range specs {
		pbSpecs[i] = &pb.VariableSpec{
			Id:               0, // MongoDB ObjectID转换为int64
			SpecProfile:      spec.SpecProfile,
			FieldKey:         spec.FieldKey,
			FieldName:        spec.FieldName,
			FieldKind:        spec.FieldKind,
			ProviderKey:      spec.ProviderKey,
			UiControl:        spec.UIControl,
			Dependencies:     marshalDependencies(spec.Dependencies),
			AllowManualInput: spec.AllowManualInput,
			IsRequired:       spec.IsRequired,
			DefaultValue:     spec.DefaultValue,
			Placeholder:      spec.Placeholder,
			HelpText:         spec.HelpText,
			SortOrder:        int32(spec.SortOrder),
		}
	}

	return &pb.GetChannelVariableSpecsResp{
		Specs: pbSpecs,
	}, nil
}

// getSpecProfileByChannelType 根据渠道类型获取规格模板（临时映射）
func getSpecProfileByChannelType(channelTypeCode string) string {
	mapping := map[string]string{
		"gitlab":     "git",
		"github":     "git",
		"gitee":      "git",
		"harbor":     "image_registry",
		"kubernetes": "k8s",
		"host_group": "host_group",
	}
	return mapping[channelTypeCode]
}

// marshalDependencies 序列化依赖规则
func marshalDependencies(deps interface{}) string {
	if deps == nil {
		return ""
	}

	// MongoDB中dependencies存储为interface{}
	jsonBytes, err := json.Marshal(deps)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}
