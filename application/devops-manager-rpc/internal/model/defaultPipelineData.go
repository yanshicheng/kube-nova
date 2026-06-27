package model

import (
	"context"
	"errors"
)

type defaultStepCategorySeed struct {
	Name        string
	Code        string
	Description string
	Icon        string
	IconColor   string
	SortOrder   int64
}

type defaultPipelineEnvironmentSeed struct {
	Name        string
	Code        string
	Description string
	Icon        string
	IconColor   string
	SortOrder   int64
}

type defaultStepTemplateSeed struct {
	Name         string
	Code         string
	Description  string
	CategoryCode string
	Icon         string
	IconColor    string
	StageContent string
	Params       []DevopsStepParam
}

var defaultStepCategories = []defaultStepCategorySeed{
	{Name: "代码拉取", Code: "code", Description: "代码仓库检出与源码准备", Icon: "ri:git-repository-line", IconColor: "#f97316", SortOrder: 10},
	{Name: "构建打包", Code: "build", Description: "语言构建、依赖安装与制品打包", Icon: "ri:hammer-line", IconColor: "#2563eb", SortOrder: 20},
	{Name: "镜像构建", Code: "image", Description: "容器镜像构建与推送", Icon: "simple-icons:docker", IconColor: "#0ea5e9", SortOrder: 30},
	{Name: "质量扫描", Code: "scan", Description: "代码质量、漏洞与安全扫描", Icon: "ri:scan-line", IconColor: "#4b9fd5", SortOrder: 40},
	{Name: "部署发布", Code: "deploy", Description: "应用部署、发布与回滚", Icon: "ri:rocket-line", IconColor: "#7c3aed", SortOrder: 50},
	{Name: "确认分类", Code: "approval", Description: "人工确认、审批与卡点控制", Icon: "ri:user-follow-line", IconColor: "#d97706", SortOrder: 60},
	{Name: "通用步骤", Code: "common", Description: "Shell、归档、清理等通用操作", Icon: "ri:tools-line", IconColor: "#64748b", SortOrder: 70},
	{Name: "Post 步骤", Code: "post", Description: "流水线 post 阶段清理、通知与收尾动作", Icon: "ri:logout-box-r-line", IconColor: "#f97316", SortOrder: 80},
}

var defaultPipelineEnvironments = []defaultPipelineEnvironmentSeed{
	{Name: "开发环境", Code: "dev", Description: "日常开发联调环境", Icon: "ri:code-s-slash-line", IconColor: "#2563eb", SortOrder: 10},
	{Name: "测试环境", Code: "test", Description: "测试验证环境", Icon: "ri:test-tube-line", IconColor: "#0f766e", SortOrder: 20},
	{Name: "准生产环境", Code: "staging", Description: "生产发布前预验证环境", Icon: "ri:rocket-2-line", IconColor: "#7c3aed", SortOrder: 30},
	{Name: "灾备环境", Code: "dr", Description: "灾备恢复和容灾演练环境", Icon: "ri:shield-flash-line", IconColor: "#ea580c", SortOrder: 40},
	{Name: "生产环境", Code: "prod", Description: "线上生产环境", Icon: "ri:building-4-line", IconColor: "#dc2626", SortOrder: 50},
}

var defaultTektonStepTemplates = []defaultStepTemplateSeed{
	{
		Name:         "Git Clone Java",
		Code:         "git-clone-java",
		Description:  "克隆 Git 仓库到 output workspace，支持可选 basic-auth Secret。",
		CategoryCode: "code",
		Icon:         "ri:git-repository-line",
		IconColor:    "#f97316",
		StageContent: defaultTektonGitCloneJavaTaskYAML,
		Params: []DevopsStepParam{
			{Name: "仓库地址", Code: "url", ParamType: "gitRepositoryUrl", Mode: "params", RuntimeMode: "params", Required: true, RuntimeConfig: true, Description: "Git 仓库 HTTP/HTTPS 地址", SortOrder: 1},
			{Name: "版本", Code: "revision", ParamType: "gitBranch", Mode: "params", RuntimeMode: "params", DefaultValue: "main", RuntimeConfig: true, Description: "分支、标签或提交哈希", SortOrder: 2},
			{Name: "克隆深度", Code: "depth", ParamType: "number", Mode: "params", RuntimeMode: "params", DefaultValue: "1", Description: "浅克隆深度", SortOrder: 3},
			{Name: "BasicAuth Secret", Code: "basicAuthSecretName", ParamType: "kubernetesSecretName", Mode: "params", RuntimeMode: "params", Description: "项目命名空间中的 Kubernetes Secret 名称", SortOrder: 4, Config: DevopsStepParamConfig{MappingField: "basic-auth", FullRow: true}},
		},
	},
}

const defaultTektonGitCloneJavaTaskYAML = `apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: git-clone-java
  labels:
    app.kubernetes.io/version: "0.9"
  annotations:
    tekton.dev/pipelines.minVersion: "0.38.0"
    tekton.dev/categories: Git
    tekton.dev/tags: git
    tekton.dev/displayName: "git clone"
    tekton.dev/platforms: "linux/amd64,linux/s390x,linux/ppc64le,linux/arm64"
spec:
  description: >-
    克隆 Git 仓库到 output workspace，支持 HTTP/HTTPS BasicAuth Secret。
  workspaces:
    - name: output
      description: Git 仓库将被克隆到该工作区。
    - name: basic-auth
      optional: true
      description: 包含 username 和 password 文件的 Secret workspace。
  params:
    - name: url
      description: 克隆仓库的 URL。
      type: string
    - name: revision
      description: 要检出的修订版本。
      type: string
      default: main
    - name: depth
      description: 浅克隆深度。
      type: string
      default: "1"
    - name: sslVerify
      description: 是否启用 Git SSL 校验。
      type: string
      default: "false"
    - name: subdirectory
      description: 在 output 工作区中克隆仓库的子目录。
      type: string
      default: ""
    - name: deleteExisting
      description: 克隆前是否清空目标目录。
      type: string
      default: "true"
    - name: userHome
      description: 用户主目录。
      type: string
      default: /root
  results:
    - name: commit
      description: 当前提交 SHA。
    - name: url
      description: 当前仓库 URL。
    - name: committer-date
      description: 提交时间戳。
  steps:
    - name: clone
      image: harbor.ikubeops.local/istio/git:2.46.0-debian-12-r3
      env:
        - name: HOME
          value: $(params.userHome)
        - name: PARAM_URL
          value: $(params.url)
        - name: PARAM_REVISION
          value: $(params.revision)
        - name: PARAM_DEPTH
          value: $(params.depth)
        - name: PARAM_SSL_VERIFY
          value: $(params.sslVerify)
        - name: PARAM_SUBDIRECTORY
          value: $(params.subdirectory)
        - name: PARAM_DELETE_EXISTING
          value: $(params.deleteExisting)
        - name: PARAM_USER_HOME
          value: $(params.userHome)
        - name: WORKSPACE_OUTPUT_PATH
          value: $(workspaces.output.path)
        - name: WORKSPACE_BASIC_AUTH_BOUND
          value: $(workspaces.basic-auth.bound)
        - name: WORKSPACE_BASIC_AUTH_PATH
          value: $(workspaces.basic-auth.path)
      securityContext:
        runAsNonRoot: false
        runAsUser: 0
      script: |
        #!/usr/bin/env sh
        set -eu

        USERNAME=""
        PASSWORD=""
        if [ "${WORKSPACE_BASIC_AUTH_BOUND}" = "true" ]; then
          if [ -f "${WORKSPACE_BASIC_AUTH_PATH}/username" ] && [ -f "${WORKSPACE_BASIC_AUTH_PATH}/password" ]; then
            USERNAME="$(cat "${WORKSPACE_BASIC_AUTH_PATH}/username")"
            PASSWORD="$(cat "${WORKSPACE_BASIC_AUTH_PATH}/password")"
          else
            echo "basic-auth Secret 缺少 username 或 password 文件"
            exit 1
          fi
        fi

        CHECKOUT_DIR="${WORKSPACE_OUTPUT_PATH}/${PARAM_SUBDIRECTORY}"
        if [ "${PARAM_DELETE_EXISTING}" = "true" ] && [ -d "${CHECKOUT_DIR}" ]; then
          rm -rf "${CHECKOUT_DIR:?}"/*
          rm -rf "${CHECKOUT_DIR}"/.[!.]* || true
          rm -rf "${CHECKOUT_DIR}"/..?* || true
        fi

        if [ "${PARAM_SSL_VERIFY}" != "true" ]; then
          git config --global http.sslVerify false
        fi
        git config --global --add safe.directory "${WORKSPACE_OUTPUT_PATH}"

        CLONE_URL="${PARAM_URL}"
        if [ -n "${USERNAME}" ] && [ -n "${PASSWORD}" ]; then
          ENCODED_USERNAME="$(printf "%s" "${USERNAME}" | sed -e 's/@/%40/g' -e 's/:/%3A/g')"
          ENCODED_PASSWORD="$(printf "%s" "${PASSWORD}" | sed -e 's/@/%40/g' -e 's/:/%3A/g')"
          case "${PARAM_URL}" in
            https://*) CLONE_URL="https://${ENCODED_USERNAME}:${ENCODED_PASSWORD}@${PARAM_URL#https://}" ;;
            http://*) CLONE_URL="http://${ENCODED_USERNAME}:${ENCODED_PASSWORD}@${PARAM_URL#http://}" ;;
            *) echo "BasicAuth 仅支持 HTTP/HTTPS 仓库地址"; exit 1 ;;
          esac
        fi

        if [ -n "${PARAM_REVISION}" ]; then
          git clone --depth="${PARAM_DEPTH}" --branch "${PARAM_REVISION}" "${CLONE_URL}" "${CHECKOUT_DIR}"
        else
          git clone --depth="${PARAM_DEPTH}" "${CLONE_URL}" "${CHECKOUT_DIR}"
        fi

        cd "${CHECKOUT_DIR}"
        RESULT_SHA="$(git rev-parse HEAD)"
        RESULT_COMMITTER_DATE="$(git log -1 --pretty=%ct)"
        printf "%s" "${RESULT_COMMITTER_DATE}" > "$(results.committer-date.path)"
        printf "%s" "${RESULT_SHA}" > "$(results.commit.path)"
        printf "%s" "${PARAM_URL}" > "$(results.url.path)"
`

func EnsureDefaultPipelineData(ctx context.Context, categoryModel *DevopsStepCategoryModel, environmentModel *DevopsPipelineEnvironmentModel, stepTemplateModel *DevopsStepTemplateModel) error {
	for _, item := range defaultStepCategories {
		exist, err := categoryModel.FindOneByCode(ctx, item.Code)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return err
			}
			data := &DevopsStepCategory{
				Name:        item.Name,
				Code:        item.Code,
				Description: item.Description,
				Icon:        item.Icon,
				IconColor:   item.IconColor,
				SortOrder:   item.SortOrder,
				Status:      1,
				CreatedBy:   "system",
				UpdatedBy:   "system",
			}
			if err := categoryModel.Insert(ctx, data); err != nil && !isDuplicateKey(err) {
				return err
			}
			continue
		}
		_ = exist
	}
	if err := ensureDefaultTektonStepTemplates(ctx, categoryModel, stepTemplateModel); err != nil {
		return err
	}
	if environmentModel == nil {
		return nil
	}
	if err := environmentModel.UnsetLegacyScopeProject(ctx); err != nil {
		return err
	}
	for _, item := range defaultPipelineEnvironments {
		exist, err := environmentModel.FindOneByCode(ctx, item.Code)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return err
			}
			data := &DevopsPipelineEnvironment{
				Name:        item.Name,
				Code:        item.Code,
				Description: item.Description,
				Icon:        item.Icon,
				IconColor:   item.IconColor,
				SortOrder:   item.SortOrder,
				Status:      1,
				CreatedBy:   "system",
				UpdatedBy:   "system",
			}
			if err := environmentModel.Insert(ctx, data); err != nil && !isDuplicateKey(err) {
				return err
			}
			continue
		}
		_ = exist
	}
	return nil
}

func ensureDefaultTektonStepTemplates(ctx context.Context, categoryModel *DevopsStepCategoryModel, stepTemplateModel *DevopsStepTemplateModel) error {
	if stepTemplateModel == nil {
		return nil
	}
	for _, item := range defaultTektonStepTemplates {
		if _, err := stepTemplateModel.FindOneByCode(ctx, "tekton", item.Code); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		categoryID := ""
		if category, err := categoryModel.FindOneByCode(ctx, item.CategoryCode); err == nil {
			categoryID = category.ID.Hex()
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		data := &DevopsStepTemplate{
			Name:                   item.Name,
			Code:                   item.Code,
			Icon:                   item.Icon,
			IconColor:              item.IconColor,
			Description:            item.Description,
			Type:                   "task",
			CategoryID:             categoryID,
			EngineType:             "tekton",
			EngineChannelGroupCode: BuildChannelGroupCode,
			EngineChannelType:      "tekton",
			StageContent:           item.StageContent,
			Params:                 item.Params,
			Status:                 1,
			CreatedBy:              "system",
			UpdatedBy:              "system",
		}
		if err := stepTemplateModel.Insert(ctx, data); err != nil && !isDuplicateKey(err) {
			return err
		}
	}
	return nil
}
