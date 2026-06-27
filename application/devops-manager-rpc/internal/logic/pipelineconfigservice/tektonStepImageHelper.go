package pipelineconfigservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	yamlv3 "gopkg.in/yaml.v3"
)

const (
	tektonStepImageTypeImage = "image"
	tektonStepImageTypeParam = "param"
)

type tektonStepImageItem struct {
	ID        string
	Name      string
	Code      string
	Image     string
	ImageType string
	ParamName string
}

type tektonStepImageReplacement struct {
	Image     string
	NewImage  string
	ImageType string
	ParamName string
}

func collectTektonStepImagesFromTemplate(step *model.DevopsStepTemplate) ([]tektonStepImageItem, error) {
	if step == nil {
		return nil, nil
	}
	root, _, err := parseTektonStepImageYaml(step.StageContent)
	if err != nil {
		return nil, err
	}
	paramDefaults := tektonTaskParamDefaultValues(root)
	imageNodes := tektonTaskContainerImageNodes(root)
	result := make([]tektonStepImageItem, 0, len(imageNodes))
	seen := make(map[string]struct{}, len(imageNodes))
	for _, node := range imageNodes {
		imageRef := strings.TrimSpace(node.Value)
		if imageRef == "" {
			continue
		}
		item := tektonStepImageItem{
			ID:        step.ID.Hex(),
			Name:      step.Name,
			Code:      step.Code,
			Image:     imageRef,
			ImageType: tektonStepImageTypeImage,
		}
		if paramName, ok := tektonImageParamRef(imageRef); ok {
			item.Image = paramDefaults[paramName]
			item.ImageType = tektonStepImageTypeParam
			item.ParamName = paramName
		}
		key := item.ID + "|" + item.ImageType + "|" + item.ParamName + "|" + item.Image
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result, nil
}

func replaceTektonStepImages(content string, replacements []tektonStepImageReplacement) (string, bool, error) {
	root, doc, err := parseTektonStepImageYaml(content)
	if err != nil {
		return "", false, errorx.Msg(err.Error())
	}
	changed := false
	for _, item := range replacements {
		switch item.ImageType {
		case tektonStepImageTypeParam:
			ok, err := replaceTektonParamImage(root, item)
			if err != nil {
				return "", false, err
			}
			changed = changed || ok
		case tektonStepImageTypeImage:
			if replaceTektonDirectImage(root, item.Image, item.NewImage) {
				changed = true
			}
		default:
			return "", false, errorx.Msg("镜像类型不支持")
		}
	}
	if !changed {
		return strings.TrimSpace(content), false, nil
	}
	data, err := yamlv3.Marshal(doc)
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(data)), true, nil
}

func buildTektonStepImageUpdateData(ctx context.Context, svcCtx *svc.ServiceContext, step *model.DevopsStepTemplate, content string, updatedBy string) (*model.DevopsStepTemplate, error) {
	stageContent, err := validateTektonStepContent(content, step.Code)
	if err != nil {
		return nil, err
	}
	if step.CategoryID != "" {
		category, err := svcCtx.StepCategoryModel.FindOne(ctx, step.CategoryID)
		if err != nil {
			return nil, err
		}
		if category.Status != 1 {
			return nil, errorx.Msg("步骤分类已停用")
		}
	}
	taskParams, taskResults, taskWorkspaces, err := tektonTaskContractsFromContent(stageContent)
	if err != nil {
		return nil, err
	}
	taskParams = mergeTektonTaskParamStepContracts(taskParams, stepParamsToPb(step.Params))
	taskParams = mergeTektonTaskParamContracts(taskParams, tektonTaskParamsToPb(step.TaskParams))
	params := stepParamsFromTektonTaskParams(taskParams, step.Params)
	if err := validateStepParams(ctx, svcCtx, tektonEngineType, step.ID.Hex(), params); err != nil {
		return nil, err
	}
	if err := validateTektonStepParamMappings(stageContent, params); err != nil {
		return nil, err
	}
	if err := validateStepArtifactConfig(step.ArtifactConfig); err != nil {
		return nil, err
	}
	return &model.DevopsStepTemplate{
		ID:                     step.ID,
		Name:                   step.Name,
		Code:                   step.Code,
		Icon:                   step.Icon,
		IconColor:              step.IconColor,
		Description:            step.Description,
		Type:                   normalizeStepType(step.Type),
		CategoryID:             step.CategoryID,
		EngineType:             tektonEngineType,
		EngineChannelGroupCode: tektonEngineChannelGroupCode,
		EngineChannelType:      tektonEngineChannelType,
		StageContent:           stageContent,
		Params:                 params,
		TaskParams:             taskParams,
		TaskResults:            taskResults,
		TaskWorkspaces:         taskWorkspaces,
		ArtifactConfig:         step.ArtifactConfig,
		Status:                 step.Status,
		UpdatedBy:              updatedBy,
	}, nil
}

func tektonStepImageItemToPb(item tektonStepImageItem) *pb.TektonStepImageItem {
	return &pb.TektonStepImageItem{
		Id:        item.ID,
		Name:      item.Name,
		Code:      item.Code,
		Image:     item.Image,
		ImageType: item.ImageType,
		ParamName: item.ParamName,
	}
}

func tektonStepImageReplacementFromPb(item *pb.UpdateTektonStepImageItem) (tektonStepImageReplacement, error) {
	if item == nil {
		return tektonStepImageReplacement{}, errorx.Msg("镜像更新项不能为空")
	}
	next := tektonStepImageReplacement{
		Image:     strings.TrimSpace(item.Image),
		NewImage:  strings.TrimSpace(item.NewImage),
		ImageType: strings.TrimSpace(item.ImageType),
		ParamName: strings.TrimSpace(item.ParamName),
	}
	if next.NewImage == "" {
		return tektonStepImageReplacement{}, errorx.Msg("新镜像不能为空")
	}
	if next.ImageType == tektonStepImageTypeParam && next.ParamName == "" {
		return tektonStepImageReplacement{}, errorx.Msg("变量字段不能为空")
	}
	if next.ImageType == tektonStepImageTypeImage && next.Image == "" {
		return tektonStepImageReplacement{}, errorx.Msg("原镜像不能为空")
	}
	return next, nil
}

func parseTektonStepImageYaml(content string) (*yamlv3.Node, *yamlv3.Node, error) {
	var doc yamlv3.Node
	if err := yamlv3.Unmarshal([]byte(strings.TrimSpace(content)), &doc); err != nil {
		return nil, nil, err
	}
	root := yamlDocumentRoot(&doc)
	if root == nil || root.Kind != yamlv3.MappingNode {
		return nil, nil, errorx.Msg("Tekton YAML 根节点必须是对象")
	}
	return root, &doc, nil
}

func tektonTaskParamDefaultValues(root *yamlv3.Node) map[string]string {
	result := make(map[string]string)
	for name, node := range tektonTaskParamDefaultNodes(root) {
		if node == nil || node.Kind != yamlv3.ScalarNode {
			continue
		}
		result[name] = strings.TrimSpace(node.Value)
	}
	return result
}

func tektonTaskParamDefaultNodes(root *yamlv3.Node) map[string]*yamlv3.Node {
	result := make(map[string]*yamlv3.Node)
	params := yamlMappingPath(root, "spec", "params")
	if params == nil || params.Kind != yamlv3.SequenceNode {
		return result
	}
	for _, item := range params.Content {
		if item == nil || item.Kind != yamlv3.MappingNode {
			continue
		}
		nameNode := yamlMappingValue(item, "name")
		if nameNode == nil || nameNode.Kind != yamlv3.ScalarNode {
			continue
		}
		name := strings.TrimSpace(nameNode.Value)
		if name == "" {
			continue
		}
		result[name] = yamlMappingValue(item, "default")
	}
	return result
}

func tektonTaskParamNodes(root *yamlv3.Node) map[string]*yamlv3.Node {
	result := make(map[string]*yamlv3.Node)
	params := yamlMappingPath(root, "spec", "params")
	if params == nil || params.Kind != yamlv3.SequenceNode {
		return result
	}
	for _, item := range params.Content {
		if item == nil || item.Kind != yamlv3.MappingNode {
			continue
		}
		nameNode := yamlMappingValue(item, "name")
		if nameNode == nil || nameNode.Kind != yamlv3.ScalarNode {
			continue
		}
		name := strings.TrimSpace(nameNode.Value)
		if name != "" {
			result[name] = item
		}
	}
	return result
}

func tektonTaskContainerImageNodes(root *yamlv3.Node) []*yamlv3.Node {
	result := make([]*yamlv3.Node, 0)
	for _, field := range []string{"steps", "sidecars"} {
		items := yamlMappingPath(root, "spec", field)
		if items == nil || items.Kind != yamlv3.SequenceNode {
			continue
		}
		for _, item := range items.Content {
			if item == nil || item.Kind != yamlv3.MappingNode {
				continue
			}
			imageNode := yamlMappingValue(item, "image")
			if imageNode != nil && imageNode.Kind == yamlv3.ScalarNode {
				result = append(result, imageNode)
			}
		}
	}
	return result
}

func replaceTektonParamImage(root *yamlv3.Node, replacement tektonStepImageReplacement) (bool, error) {
	paramName := replacement.ParamName
	paramNodes := tektonTaskParamNodes(root)
	paramNode := paramNodes[paramName]
	if paramNode == nil {
		return false, errorx.Msg("未找到变量字段：" + paramName)
	}
	ref := "$(params." + paramName + ")"
	referenced := false
	for _, node := range tektonTaskContainerImageNodes(root) {
		if strings.TrimSpace(node.Value) == ref {
			referenced = true
			break
		}
	}
	if !referenced {
		return false, errorx.Msg("未找到变量字段镜像引用：" + paramName)
	}
	defaultNode := yamlMappingValue(paramNode, "default")
	if defaultNode != nil && defaultNode.Kind == yamlv3.ScalarNode && strings.TrimSpace(defaultNode.Value) == replacement.NewImage {
		return false, nil
	}
	yamlSetScalar(paramNode, "default", replacement.NewImage)
	return true, nil
}

func replaceTektonDirectImage(root *yamlv3.Node, oldImage, newImage string) bool {
	changed := false
	for _, node := range tektonTaskContainerImageNodes(root) {
		if strings.TrimSpace(node.Value) != oldImage {
			continue
		}
		node.Value = newImage
		node.Tag = "!!str"
		changed = true
	}
	return changed
}

func tektonImageParamRef(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "$(params.") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(value, "$(params."), ")")
	if name == "" || !tektonParamNamePattern.MatchString(name) {
		return "", false
	}
	return name, true
}

func yamlMappingPath(node *yamlv3.Node, keys ...string) *yamlv3.Node {
	current := node
	for _, key := range keys {
		current = yamlMappingValue(current, key)
		if current == nil {
			return nil
		}
	}
	return current
}
