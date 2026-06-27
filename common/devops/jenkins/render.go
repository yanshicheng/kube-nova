package jenkins

import (
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type RenderPipeline struct {
	Agent       JenkinsRenderAgent
	Tools       []JenkinsRenderKV
	Options     []JenkinsRenderKV
	Triggers    []JenkinsRenderTrigger
	Environment map[string]string
	Params      []JenkinsRenderParam
	Steps       []JenkinsRenderStep
}

type RenderResult struct {
	Pipeline   string
	StageNames map[string]string
}

type JenkinsRenderAgent struct {
	Type        string
	Label       string
	DockerImage string
	DockerArgs  string
	Raw         string
	ID          string
	Name        string
	AgentType   string
	MatchMode   string
	MatchValue  string
	Cloud       string
	PodYaml     string
}

type JenkinsRenderKV struct {
	Name  string
	Value string
	Type  string
}

type JenkinsRenderTrigger struct {
	Type    string
	Spec    string
	Enabled bool
}

type JenkinsRenderParam struct {
	Code         string
	DefaultValue string
	CurrentValue string
	ParamType    string
	Mode         string
	RuntimeMode  string
	Required     bool
	Readonly     bool
	Description  string
	SelectList   []JenkinsRenderParamOption
}

type JenkinsRenderParamOption struct {
	Label string
	Value string
}

type JenkinsRenderStep struct {
	ID            string
	NodeName      string
	ContainerName string
	StepType      string
	ParentNodeID  string
	BranchType    string
	SortOrder     int64
	Enabled       bool
	StageContent  string
	ParamCodeMap  map[string]string
	Artifact      JenkinsRenderArtifact
}

type JenkinsRenderArtifact struct {
	Enabled     bool
	Type        string
	Name        string
	Path        string
	Required    bool
	ContentType string
}

var stageNamePattern = regexp.MustCompile(`stage\s*\(\s*(['"])([^'"]*)(['"])\s*\)`)
var postConditionPattern = regexp.MustCompile(`^\s*(always|success|failure|changed|aborted|unstable)\s*\{`)
var postConditionOrder = []string{"always", "success", "failure", "changed", "aborted", "unstable"}

func RenderDeclarativePipeline(input RenderPipeline) (string, error) {
	result, err := RenderDeclarativePipelineWithMetadata(input)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return "", err
	}
	return result.Pipeline, nil
}

func RenderDeclarativePipelineWithMetadata(input RenderPipeline) (RenderResult, error) {
	passwordParams := passwordParamSet(input.Params)
	stageNames := make(map[string]string)
	var b strings.Builder
	b.WriteString("pipeline {\n")
	writeAgent(&b, input.Agent, 1)
	writeTools(&b, input.Tools, 1)
	writeOptions(&b, input.Options, 1)
	writeTriggers(&b, input.Triggers, 1)
	if err := writeParameters(&b, input.Params, 1); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return RenderResult{}, err
	}
	writeEnvironment(&b, input.Environment, 1)
	b.WriteString(indent(1) + "stages {\n")
	if err := writeStageTree(&b, input.Steps, "workflow-start", 2, map[string]int{}, passwordParams, stageNames); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return RenderResult{}, err
	}
	b.WriteString(indent(1) + "}\n")
	postSteps := childrenOf(input.Steps, "workflow-post")
	if len(postSteps) > 0 {
		if err := writePostSteps(&b, postSteps, 1); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return RenderResult{}, err
		}
	}
	b.WriteString("}\n")
	return RenderResult{Pipeline: b.String(), StageNames: stageNames}, nil
}

func writeAgent(b *strings.Builder, agent JenkinsRenderAgent, level int) {
	switch strings.TrimSpace(agent.Type) {
	case "", "any":
		b.WriteString(indent(level) + "agent any\n")
	case "none":
		b.WriteString(indent(level) + "agent none\n")
	case "static":
		writeConfiguredStaticAgent(b, agent, level)
	case "dynamic", "pod":
		writeConfiguredPodAgent(b, agent, level)
	case "label":
		b.WriteString(fmt.Sprintf("%sagent { label '%s' }\n", indent(level), singleQuote(agent.Label)))
	case "docker":
		b.WriteString(indent(level) + "agent {\n")
		b.WriteString(indent(level+1) + "docker {\n")
		b.WriteString(fmt.Sprintf("%simage '%s'\n", indent(level+2), singleQuote(agent.DockerImage)))
		if strings.TrimSpace(agent.DockerArgs) != "" {
			b.WriteString(fmt.Sprintf("%sargs '%s'\n", indent(level+2), singleQuote(agent.DockerArgs)))
		}
		b.WriteString(indent(level+1) + "}\n")
		b.WriteString(indent(level) + "}\n")
	case "raw":
		if strings.TrimSpace(agent.Raw) != "" {
			b.WriteString(indentText(agent.Raw, level))
			b.WriteString("\n")
		}
	default:
		b.WriteString(indent(level) + "agent any\n")
	}
}

func writeConfiguredStaticAgent(b *strings.Builder, agent JenkinsRenderAgent, level int) {
	value := strings.TrimSpace(agent.MatchValue)
	if value == "" {
		b.WriteString(indent(level) + "agent any\n")
		return
	}
	if strings.TrimSpace(agent.MatchMode) == "name" {
		b.WriteString(indent(level) + "agent {\n")
		b.WriteString(indent(level+1) + "node {\n")
		b.WriteString(fmt.Sprintf("%slabel '%s'\n", indent(level+2), singleQuote(value)))
		b.WriteString(indent(level+1) + "}\n")
		b.WriteString(indent(level) + "}\n")
		return
	}
	b.WriteString(fmt.Sprintf("%sagent { label '%s' }\n", indent(level), singleQuote(value)))
}

func writeConfiguredPodAgent(b *strings.Builder, agent JenkinsRenderAgent, level int) {
	if strings.TrimSpace(agent.PodYaml) != "" || strings.TrimSpace(agent.Cloud) != "" {
		b.WriteString(indent(level) + "agent {\n")
		b.WriteString(indent(level+1) + "kubernetes {\n")
		if strings.TrimSpace(agent.Cloud) != "" {
			b.WriteString(fmt.Sprintf("%scloud '%s'\n", indent(level+2), singleQuote(agent.Cloud)))
		}
		if strings.TrimSpace(agent.PodYaml) != "" {
			b.WriteString(indent(level+2) + "yaml \"\"\"\n")
			b.WriteString(indentText(agent.PodYaml, level+2))
			b.WriteString("\n")
			b.WriteString(indent(level+2) + "\"\"\"\n")
		}
		b.WriteString(indent(level+1) + "}\n")
		b.WriteString(indent(level) + "}\n")
		return
	}
	writeConfiguredStaticAgent(b, agent, level)
}

func writeTools(b *strings.Builder, tools []JenkinsRenderKV, level int) {
	if len(tools) == 0 {
		return
	}
	b.WriteString(indent(level) + "tools {\n")
	for _, item := range tools {
		if item.Type == "" || item.Name == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%s%s '%s'\n", indent(level+1), item.Type, singleQuote(item.Name)))
	}
	b.WriteString(indent(level) + "}\n")
}

func writeOptions(b *strings.Builder, options []JenkinsRenderKV, level int) {
	if len(options) == 0 {
		return
	}
	b.WriteString(indent(level) + "options {\n")
	for _, item := range options {
		if item.Name == "" {
			continue
		}
		if item.Value == "" {
			b.WriteString(fmt.Sprintf("%s%s()\n", indent(level+1), item.Name))
		} else {
			b.WriteString(fmt.Sprintf("%s%s(%s)\n", indent(level+1), item.Name, item.Value))
		}
	}
	b.WriteString(indent(level) + "}\n")
}

func writeTriggers(b *strings.Builder, triggers []JenkinsRenderTrigger, level int) {
	active := make([]JenkinsRenderTrigger, 0, len(triggers))
	for _, item := range triggers {
		if item.Enabled && item.Type != "" {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		return
	}
	b.WriteString(indent(level) + "triggers {\n")
	for _, item := range active {
		switch item.Type {
		case "cron":
			b.WriteString(fmt.Sprintf("%scron('%s')\n", indent(level+1), singleQuote(item.Spec)))
		case "pollSCM":
			b.WriteString(fmt.Sprintf("%spollSCM('%s')\n", indent(level+1), singleQuote(item.Spec)))
		default:
			b.WriteString(fmt.Sprintf("%s%s\n", indent(level+1), item.Spec))
		}
	}
	b.WriteString(indent(level) + "}\n")
}

func writeParameters(b *strings.Builder, params []JenkinsRenderParam, level int) error {
	items := make([]JenkinsRenderParam, 0, len(params))
	for _, item := range params {
		if effectiveParamRuntimeMode(item) == "params" && item.Code != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}
	b.WriteString(indent(level) + "parameters {\n")
	for _, item := range items {
		value := item.CurrentValue
		if value == "" {
			value = item.DefaultValue
		}
		desc := singleQuote(item.Description)
		switch item.ParamType {
		case "booleanParam":
			if value != "true" {
				value = "false"
			}
			b.WriteString(fmt.Sprintf("%sbooleanParam(name: '%s', defaultValue: %s, description: '%s')\n", indent(level+1), singleQuote(item.Code), value, desc))
		case "number":
			if value != "" {
				if _, err := strconv.ParseFloat(value, 64); err != nil {
					logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", fmt.Errorf("number 参数「%s」默认值必须是合法数字", item.Code))
					return fmt.Errorf("number 参数「%s」默认值必须是合法数字", item.Code)
				}
			}
			b.WriteString(fmt.Sprintf("%sstring(name: '%s', defaultValue: '%s', description: '%s', trim: true)\n", indent(level+1), singleQuote(item.Code), singleQuote(value), desc))
		case "text":
			b.WriteString(fmt.Sprintf("%stext(name: '%s', defaultValue: '''%s''', description: '%s')\n", indent(level+1), singleQuote(item.Code), tripleSingleQuote(value), desc))
		case "password":
			b.WriteString(fmt.Sprintf("%spassword(name: '%s', defaultValue: '', description: '%s')\n", indent(level+1), singleQuote(item.Code), desc))
		case "file":
			b.WriteString(fmt.Sprintf("%sfile(name: '%s', description: '%s')\n", indent(level+1), singleQuote(item.Code), desc))
		case "choice":
			choices := renderParamChoices(item)
			if len(choices) == 0 {
				logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", fmt.Errorf("choice 参数「%s」必须配置选项", item.Code))
				return fmt.Errorf("choice 参数「%s」必须配置选项", item.Code)
			}
			b.WriteString(fmt.Sprintf("%schoice(name: '%s', choices: [%s], description: '%s')\n", indent(level+1), singleQuote(item.Code), quoteChoices(choices), desc))
		default:
			b.WriteString(fmt.Sprintf("%sstring(name: '%s', defaultValue: '%s', description: '%s', trim: true)\n", indent(level+1), singleQuote(item.Code), singleQuote(value), desc))
		}
	}
	b.WriteString(indent(level) + "}\n")
	return nil
}

func writeEnvironment(b *strings.Builder, env map[string]string, level int) {
	if len(env) == 0 {
		return
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	b.WriteString(indent(level) + "environment {\n")
	for _, key := range keys {
		value := env[key]
		if strings.Contains(value, "\n") {
			b.WriteString(fmt.Sprintf("%s%s = '''%s'''\n", indent(level+1), key, tripleSingleQuote(value)))
			continue
		}
		b.WriteString(fmt.Sprintf("%s%s = '%s'\n", indent(level+1), key, singleQuote(value)))
	}
	b.WriteString(indent(level) + "}\n")
}

func writeStageTree(b *strings.Builder, steps []JenkinsRenderStep, parentID string, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	children := childrenOf(steps, parentID)
	if len(children) == 0 {
		return nil
	}
	if hasExplicitBranchType(children) {
		return writeExplicitStageTree(b, steps, parentID, children, level, stageCount, passwordParams, stageNames)
	}
	if len(children) == 1 {
		if err := writeSingleStage(b, children[0], level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		return writeStageTree(b, steps, children[0].ID, level, stageCount, passwordParams, stageNames)
	}
	if shouldUseScriptedParallelGroup(steps, children) {
		return writeScriptedParallelStage(b, steps, parentID, level, stageCount, passwordParams, stageNames)
	}
	return writeDeclarativeParallelStage(b, steps, parentID, children, level, stageCount, passwordParams, stageNames)
}

func writeExplicitStageTree(b *strings.Builder, steps []JenkinsRenderStep, parentID string, children []JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	parallelChildren, nextChildren := splitBranchChildren(children)
	if len(parallelChildren) > 0 {
		nextChildren = appendUniqueRenderSteps(nextChildren, parallelContinuationRoots(steps, parallelChildren)...)
	}
	if len(parallelChildren) == 1 {
		if err := writeSingleStage(b, parallelChildren[0], level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		if err := writeStageTree(b, steps, parallelChildren[0].ID, level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
	}
	if len(parallelChildren) > 1 {
		if shouldUseScriptedParallelGroup(steps, parallelChildren) {
			if err := writeScriptedParallelStageForChildren(b, steps, parentID, parallelChildren, level, stageCount, passwordParams, stageNames); err != nil {
				logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
				return err
			}
		} else if err := writeDeclarativeParallelStage(b, steps, parentID, parallelChildren, level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
	}
	for _, child := range nextChildren {
		if err := writeSingleStage(b, child, level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		if err := writeStageTree(b, steps, child.ID, level, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
	}
	return nil
}

func writeDeclarativeParallelStage(b *strings.Builder, steps []JenkinsRenderStep, parentID string, children []JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	name := uniqueStageName(nextParallelGroupStageName(stageCount), stageCount)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	b.WriteString(indent(level+1) + "parallel {\n")
	for _, child := range children {
		if err := writeParallelBranch(b, steps, child, level+2, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
	}
	b.WriteString(indent(level+1) + "}\n")
	b.WriteString(indent(level) + "}\n")
	return nil
}

func shouldUseScriptedParallelGroup(steps []JenkinsRenderStep, children []JenkinsRenderStep) bool {
	for _, child := range children {
		if hasParallelDescendant(steps, child.ID) {
			return true
		}
	}
	return false
}

func writeScriptedParallelStage(b *strings.Builder, steps []JenkinsRenderStep, parentID string, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	return writeScriptedParallelStageForChildren(b, steps, parentID, childrenOf(steps, parentID), level, stageCount, passwordParams, stageNames)
}

func writeScriptedParallelStageForChildren(b *strings.Builder, steps []JenkinsRenderStep, parentID string, children []JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	name := uniqueStageName(nextParallelGroupStageName(stageCount), stageCount)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	b.WriteString(indent(level+1) + "steps {\n")
	b.WriteString(indent(level+2) + "script {\n")
	if err := writeScriptedChildrenList(b, steps, children, level+3, stageCount, passwordParams, stageNames); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return err
	}
	b.WriteString(indent(level+2) + "}\n")
	b.WriteString(indent(level+1) + "}\n")
	b.WriteString(indent(level) + "}\n")
	return nil
}

func writeParallelBranch(b *strings.Builder, steps []JenkinsRenderStep, step JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	children := childrenOf(steps, step.ID)
	parallelChildren, nextChildren := splitBranchChildren(children)
	if len(nextChildren) > 0 {
		children = parallelChildren
	}
	if len(children) == 0 {
		return writeSingleStage(b, step, level, stageCount, passwordParams, stageNames)
	}
	if hasParallelDescendant(steps, step.ID) {
		return writeScriptedParallelBranch(b, steps, step, level, stageCount, passwordParams, stageNames)
	}
	name := uniqueStageName(branchStageName(step), stageCount)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	b.WriteString(indent(level+1) + "stages {\n")
	if err := writeSingleStage(b, step, level+2, stageCount, passwordParams, stageNames); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return err
	}
	if err := writeStageTree(b, steps, step.ID, level+2, stageCount, passwordParams, stageNames); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return err
	}
	b.WriteString(indent(level+1) + "}\n")
	b.WriteString(indent(level) + "}\n")
	return nil
}

func writeScriptedParallelBranch(b *strings.Builder, steps []JenkinsRenderStep, step JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	content := strings.TrimSpace(step.StageContent)
	name := uniqueStageName(stageNameForNode(step), stageCount)
	recordStageName(stageNames, step, name)
	body, err := stepsBodyContent(content)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return fmt.Errorf("阶段「%s」%w", name, err)
	}
	body = rewriteStepParamReferences(body, step.ParamCodeMap)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	b.WriteString(indent(level+1) + "steps {\n")
	writeStepBody(
		b,
		wrapStepBodyWithContainer(
			normalizePasswordParamUsage(body, passwordParams),
			step.ContainerName,
			0,
		),
		name,
		level+2,
	)
	children := childrenOf(steps, step.ID)
	if len(children) > 0 {
		b.WriteString(indent(level+2) + "script {\n")
		if err := writeScriptedChildren(b, steps, step.ID, level+3, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		b.WriteString(indent(level+2) + "}\n")
	}
	b.WriteString(indent(level+1) + "}\n")
	writeStageArtifactPost(b, step.Artifact, level+1)
	b.WriteString(indent(level) + "}\n")
	return nil
}

func writeScriptedChildren(b *strings.Builder, steps []JenkinsRenderStep, parentID string, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	children := childrenOf(steps, parentID)
	if len(children) == 0 {
		return nil
	}
	if hasExplicitBranchType(children) {
		parallelChildren, nextChildren := splitBranchChildren(children)
		if len(parallelChildren) > 0 {
			if err := writeScriptedChildrenList(b, steps, parallelChildren, level, stageCount, passwordParams, stageNames); err != nil {
				logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
				return err
			}
		}
		for _, child := range nextChildren {
			if err := writeScriptedNode(b, steps, child, level, stageCount, passwordParams, stageNames); err != nil {
				logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
				return err
			}
		}
		return nil
	}
	return writeScriptedChildrenList(b, steps, children, level, stageCount, passwordParams, stageNames)
}

func writeScriptedChildrenList(b *strings.Builder, steps []JenkinsRenderStep, children []JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	if len(children) == 1 {
		return writeScriptedNode(b, steps, children[0], level, stageCount, passwordParams, stageNames)
	}
	b.WriteString(indent(level) + "parallel(\n")
	labels := map[string]int{}
	for index, child := range children {
		label := uniqueLocalName(stageNameForNode(child), labels)
		b.WriteString(fmt.Sprintf("%s'%s': {\n", indent(level+1), singleQuote(label)))
		if err := writeScriptedNode(b, steps, child, level+2, stageCount, passwordParams, stageNames); err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		b.WriteString(indent(level+1) + "}")
		if index < len(children)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(indent(level) + ")\n")
	return nil
}

func writeScriptedNode(b *strings.Builder, steps []JenkinsRenderStep, step JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	content := strings.TrimSpace(step.StageContent)
	name := uniqueStageName(stageNameForNode(step), stageCount)
	recordStageName(stageNames, step, name)
	body, err := stepsBodyContent(content)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return fmt.Errorf("阶段「%s」%w", name, err)
	}
	body = rewriteStepParamReferences(body, step.ParamCodeMap)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	writeStepBody(
		b,
		wrapStepBodyWithContainer(
			normalizePasswordParamUsage(scriptedStepsBody(body), passwordParams),
			step.ContainerName,
			0,
		),
		name,
		level+1,
	)
	b.WriteString(indent(level) + "}\n")
	return writeScriptedChildren(b, steps, step.ID, level, stageCount, passwordParams, stageNames)
}

func hasParallelDescendant(steps []JenkinsRenderStep, parentID string) bool {
	children := childrenOf(steps, parentID)
	if len(children) > 1 {
		return true
	}
	for _, child := range children {
		if hasParallelDescendant(steps, child.ID) {
			return true
		}
	}
	return false
}

func branchStageName(step JenkinsRenderStep) string {
	name := stageNameForNode(step)
	if name == "" {
		return "并行分支"
	}
	return name + "-分支"
}

func nextParallelGroupStageName(stageCount map[string]int) string {
	const key = "__parallel_group__"
	stageCount[key]++
	return fmt.Sprintf("并行组-%d", stageCount[key])
}

func writeSingleStage(b *strings.Builder, step JenkinsRenderStep, level int, stageCount map[string]int, passwordParams map[string]struct{}, stageNames map[string]string) error {
	content := strings.TrimSpace(step.StageContent)
	name := uniqueStageName(stageNameForNode(step), stageCount)
	recordStageName(stageNames, step, name)
	if content == "" {
		b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
		b.WriteString(indent(level+1) + "steps {\n")
		b.WriteString(indent(level+2) + "script {\n")
		b.WriteString(fmt.Sprintf("%secho '%s'\n", indent(level+3), singleQuote(name)))
		b.WriteString(indent(level+2) + "}\n")
		b.WriteString(indent(level+1) + "}\n")
		writeStageArtifactPost(b, step.Artifact, level+1)
		b.WriteString(indent(level) + "}\n")
		return nil
	}
	body, err := stepsBodyContent(content)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return fmt.Errorf("阶段「%s」%w", name, err)
	}
	body = rewriteStepParamReferences(body, step.ParamCodeMap)
	b.WriteString(fmt.Sprintf("%sstage('%s') {\n", indent(level), singleQuote(name)))
	b.WriteString(indent(level+1) + "steps {\n")
	writeStepBody(
		b,
		wrapStepBodyWithContainer(
			normalizePasswordParamUsage(body, passwordParams),
			step.ContainerName,
			0,
		),
		name,
		level+2,
	)
	b.WriteString(indent(level+1) + "}\n")
	writeStageArtifactPost(b, step.Artifact, level+1)
	b.WriteString(indent(level) + "}\n")
	return nil
}

func passwordParamSet(params []JenkinsRenderParam) map[string]struct{} {
	result := map[string]struct{}{}
	for _, item := range params {
		if effectiveParamRuntimeMode(item) == "params" && item.ParamType == "password" && strings.TrimSpace(item.Code) != "" {
			result[item.Code] = struct{}{}
		}
	}
	return result
}

func effectiveParamRuntimeMode(item JenkinsRenderParam) string {
	mode := strings.TrimSpace(item.RuntimeMode)
	if mode == "params" || mode == "env" {
		return mode
	}
	if strings.TrimSpace(item.Mode) == "env" {
		return "env"
	}
	return "params"
}

func renderParamChoices(item JenkinsRenderParam) []string {
	choices := make([]string, 0, len(item.SelectList))
	seen := map[string]struct{}{}
	for _, option := range item.SelectList {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			value = strings.TrimSpace(option.Label)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		choices = append(choices, value)
	}
	if len(choices) == 0 {
		raw := strings.ReplaceAll(item.DefaultValue, "\r\n", "\n")
		for _, part := range regexp.MustCompile(`[,\n]`).Split(raw, -1) {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			choices = append(choices, value)
		}
	}
	return choices
}

func tripleSingleQuote(value string) string {
	return strings.ReplaceAll(value, "'''", "\\'\\'\\'")
}

func quoteChoices(items []string) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, "'"+singleQuote(item)+"'")
	}
	return strings.Join(parts, ", ")
}

func writeStageArtifactPost(b *strings.Builder, artifact JenkinsRenderArtifact, level int) {
	if !artifact.Enabled || strings.TrimSpace(artifact.Path) == "" {
		return
	}
	allowEmpty := "true"
	if artifact.Required {
		allowEmpty = "false"
	}
	b.WriteString(indent(level) + "post {\n")
	b.WriteString(indent(level+1) + "always {\n")
	b.WriteString(fmt.Sprintf("%sarchiveArtifacts artifacts: '%s', allowEmptyArchive: %s, fingerprint: true\n", indent(level+2), singleQuote(artifact.Path), allowEmpty))
	b.WriteString(indent(level+1) + "}\n")
	b.WriteString(indent(level) + "}\n")
}

func normalizePasswordParamUsage(body string, passwordParams map[string]struct{}) string {
	if len(passwordParams) == 0 || strings.TrimSpace(body) == "" {
		return body
	}
	for code := range passwordParams {
		quoted := regexp.QuoteMeta(code)
		body = regexp.MustCompile(`params\.`+quoted+`\s*\?\s*params\.`+quoted+`\.length\(\)\s*:\s*0`).ReplaceAllString(body, "params."+code+" ? params."+code+".toString().length() : 0")
		body = regexp.MustCompile(`params\[['"]`+quoted+`['"]\]\s*\?\s*params\[['"]`+quoted+`['"]\]\.length\(\)\s*:\s*0`).ReplaceAllString(body, "params['"+code+"'] ? params['"+code+"'].toString().length() : 0")
		body = regexp.MustCompile(`params\.`+quoted+`\.length\(\)`).ReplaceAllString(body, "params."+code+".toString().length()")
		body = regexp.MustCompile(`params\[['"]`+quoted+`['"]\]\.length\(\)`).ReplaceAllString(body, "params['"+code+"'].toString().length()")
	}
	return body
}

func rewriteStepParamReferences(body string, codeMap map[string]string) string {
	if len(codeMap) == 0 || strings.TrimSpace(body) == "" {
		return body
	}
	codes := make([]string, 0, len(codeMap))
	for source, target := range codeMap {
		if strings.TrimSpace(source) == "" || strings.TrimSpace(target) == "" || source == target {
			continue
		}
		codes = append(codes, source)
	}
	sort.Slice(codes, func(i, j int) bool {
		return len(codes[i]) > len(codes[j])
	})
	for _, source := range codes {
		target := codeMap[source]
		quoted := regexp.QuoteMeta(source)
		if isGroovyIdentifier(source) && isGroovyIdentifier(target) {
			body = regexp.MustCompile(`\bparams\.`+quoted+`\b`).ReplaceAllStringFunc(body, func(string) string {
				return "params." + target
			})
			body = regexp.MustCompile(`\benv\.`+quoted+`\b`).ReplaceAllStringFunc(body, func(string) string {
				return "env." + target
			})
		}
		body = regexp.MustCompile(`\bparams\[['"]`+quoted+`['"]\]`).ReplaceAllStringFunc(body, func(string) string {
			return "params['" + target + "']"
		})
		body = regexp.MustCompile(`\benv\[['"]`+quoted+`['"]\]`).ReplaceAllStringFunc(body, func(string) string {
			return "env['" + target + "']"
		})
		body = regexp.MustCompile(`\$\{`+quoted+`\}`).ReplaceAllStringFunc(body, func(string) string {
			return "${" + target + "}"
		})
		if isGroovyIdentifier(source) {
			body = regexp.MustCompile(`\$`+quoted+`([^A-Za-z0-9_]|$)`).ReplaceAllStringFunc(body, func(match string) string {
				return "$" + target + strings.TrimPrefix(match, "$"+source)
			})
		}
	}
	return body
}

func isGroovyIdentifier(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	return regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(value)
}

func stepsBodyContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil
	}
	if stageLoc := stageNamePattern.FindStringIndex(content); stageLoc != nil {
		openIndex := strings.Index(content[stageLoc[1]:], "{")
		if openIndex < 0 {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", errors.New("stage 代码块不完整"))
			return "", errors.New("stage 代码块不完整")
		}
		openIndex += stageLoc[1]
		closeIndex := findMatchingBrace(content, openIndex)
		if closeIndex < 0 {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", errors.New("stage 代码块不完整"))
			return "", errors.New("stage 代码块不完整")
		}
		stageBody := strings.TrimSpace(content[openIndex+1 : closeIndex])
		if stepsBody, ok := extractNamedBlock(stageBody, "steps"); ok {
			return stepsBody, nil
		}
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", errors.New("步骤模板只能保存 steps 级别内容，完整 stage 必须包含 steps 块"))
		return "", errors.New("步骤模板只能保存 steps 级别内容，完整 stage 必须包含 steps 块")
	}
	if stepsBody, ok := extractNamedBlock(content, "steps"); ok {
		return stepsBody, nil
	}
	if strings.Contains(content, "stage(") || strings.Contains(content, "parallel {") {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", errors.New("步骤模板只能保存 steps 级别内容，不能包含 stage 或 parallel"))
		return "", errors.New("步骤模板只能保存 steps 级别内容，不能包含 stage 或 parallel")
	}
	return content, nil
}

func writeStepBody(b *strings.Builder, body, fallback string, level int) {
	if strings.TrimSpace(body) == "" {
		b.WriteString(fmt.Sprintf("%secho '%s'\n", indent(level), singleQuote(fallback)))
		return
	}
	b.WriteString(indentText(body, level))
	b.WriteString("\n")
}

func wrapStepBodyWithContainer(body, containerName string, level int) string {
	containerName = strings.TrimSpace(containerName)
	if containerName == "" {
		return body
	}
	var builder strings.Builder
	builder.WriteString(indent(level) + "container('" + singleQuote(containerName) + "') {\n")
	builder.WriteString(indentText(body, level+1))
	builder.WriteString("\n")
	builder.WriteString(indent(level) + "}")
	return builder.String()
}

func scriptedStepsBody(body string) string {
	body = strings.TrimSpace(body)
	if inner, ok := extractWholeNamedBlock(body, "script"); ok {
		return inner
	}
	return body
}

func extractWholeNamedBlock(content, name string) (string, bool) {
	pattern := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(name) + `\s*\{`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return "", false
	}
	openIndex := strings.Index(content[loc[0]:loc[1]], "{")
	if openIndex < 0 {
		return "", false
	}
	openIndex += loc[0]
	closeIndex := findMatchingBrace(content, openIndex)
	if closeIndex < 0 {
		return "", false
	}
	if strings.TrimSpace(content[closeIndex+1:]) != "" {
		return "", false
	}
	return strings.TrimSpace(content[openIndex+1 : closeIndex]), true
}

func uniqueLocalName(name string, counter map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未命名分支"
	}
	counter[name]++
	if counter[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, counter[name])
}

func writePostSteps(b *strings.Builder, steps []JenkinsRenderStep, level int) error {
	if err := ValidatePostConditionUniqueness(steps); err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return err
	}
	grouped := make(map[string][]string)
	rawBlocks := make([]string, 0)
	for _, item := range steps {
		condition, body, raw, err := postStepContent(item.StageContent, item.NodeName)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		body = rewriteStepParamReferences(body, item.ParamCodeMap)
		body = guardPostShellScript(body)
		if raw {
			rawBlocks = append(rawBlocks, body)
			continue
		}
		grouped[condition] = append(grouped[condition], body)
	}
	if len(grouped) == 0 && len(rawBlocks) == 0 {
		return nil
	}
	b.WriteString(indent(level) + "post {\n")
	written := make(map[string]struct{})
	for _, condition := range postConditionOrder {
		bodies := grouped[condition]
		if len(bodies) == 0 {
			continue
		}
		writePostCondition(b, condition, bodies, level+1)
		written[condition] = struct{}{}
	}
	for condition, bodies := range grouped {
		if _, ok := written[condition]; ok {
			continue
		}
		writePostCondition(b, condition, bodies, level+1)
	}
	for _, body := range rawBlocks {
		b.WriteString(indentText(body, level+1))
		b.WriteString("\n")
	}
	b.WriteString(indent(level) + "}\n")
	return nil
}

func ValidatePostConditionUniqueness(steps []JenkinsRenderStep) error {
	seen := make(map[string]string)
	for _, item := range steps {
		if !item.Enabled {
			continue
		}
		conditions, err := postStepConditionNames(item.StageContent)
		if err != nil {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
			return err
		}
		for _, condition := range conditions {
			if firstNode, ok := seen[condition]; ok {
				return duplicatePostConditionError(condition, firstNode, item.NodeName)
			}
			seen[condition] = strings.TrimSpace(item.NodeName)
		}
	}
	return nil
}

func writePostCondition(b *strings.Builder, condition string, bodies []string, level int) {
	b.WriteString(indent(level) + condition + " {\n")
	for _, body := range bodies {
		b.WriteString(indentText(body, level+1))
		b.WriteString("\n")
	}
	b.WriteString(indent(level) + "}\n")
}

func guardPostShellScript(body string) string {
	if strings.TrimSpace(body) == "" {
		return body
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^(\s*)sh\s+scriptContent\s*$`),
		regexp.MustCompile(`(?m)^(\s*)sh\s*\(\s*script\s*:\s*scriptContent\s*\)\s*$`),
	}
	replacement := "${1}if (scriptContent?.trim()) {\n${1}  sh scriptContent\n${1}} else {\n${1}  echo 'post 脚本为空，跳过'\n${1}}"
	for _, pattern := range patterns {
		body = pattern.ReplaceAllString(body, replacement)
	}
	return body
}

func postStepContent(content, fallback string) (string, string, bool, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "always", fmt.Sprintf("echo '%s'", singleQuote(fallback)), false, nil
	}
	if body, ok := extractNamedBlock(content, "post"); ok {
		return "", body, true, nil
	}
	if conditions, err := postConditionNamesFromBody(content); err == nil {
		if len(conditions) > 1 {
			return "", content, true, nil
		}
		body, ok := extractNamedBlock(content, conditions[0])
		if !ok {
			logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", errors.New("post 条件代码块不完整"))
			return "", "", false, errors.New("post 条件代码块不完整")
		}
		return conditions[0], body, false, nil
	} else if postConditionPattern.MatchString(content) {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return "", "", false, err
	}
	body, err := stepsBodyContent(content)
	if err != nil {
		logx.Errorf("DevOps组件执行失败: component=%s err=%v", "common/devops/jenkins/render.go", err)
		return "", "", false, err
	}
	return "always", body, false, nil
}

func postStepConditionNames(content string) ([]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{"always"}, nil
	}
	if body, ok := extractNamedBlock(content, "post"); ok {
		return postConditionNamesFromBody(body)
	}
	if postConditionPattern.MatchString(content) {
		return postConditionNamesFromBody(content)
	}
	return []string{"always"}, nil
}

func postConditionNamesFromBody(content string) ([]string, error) {
	rest := strings.TrimSpace(content)
	if rest == "" {
		return nil, errors.New("post 步骤条件不能为空")
	}
	names := make([]string, 0)
	seen := make(map[string]struct{})
	for rest != "" {
		match := postConditionPattern.FindStringSubmatchIndex(rest)
		if match == nil || match[0] != 0 {
			return nil, errors.New("post 步骤只允许 always、success、failure、changed、aborted、unstable 条件块")
		}
		condition := rest[match[2]:match[3]]
		if _, ok := seen[condition]; ok {
			return nil, duplicatePostConditionError(condition, "", "")
		}
		seen[condition] = struct{}{}
		names = append(names, condition)
		openIndex := strings.Index(rest[match[0]:match[1]], "{")
		if openIndex < 0 {
			return nil, errors.New("post 条件代码块不完整")
		}
		openIndex += match[0]
		closeIndex := findMatchingBrace(rest, openIndex)
		if closeIndex < 0 {
			return nil, errors.New("post 条件代码块不完整")
		}
		rest = strings.TrimSpace(rest[closeIndex+1:])
	}
	return names, nil
}

func duplicatePostConditionError(condition, firstNode, currentNode string) error {
	firstNode = strings.TrimSpace(firstNode)
	currentNode = strings.TrimSpace(currentNode)
	if firstNode != "" && currentNode != "" {
		return fmt.Errorf("post 条件 %s 只能配置一次，已在「%s」中配置", condition, firstNode)
	}
	return fmt.Errorf("post 条件 %s 只能配置一次", condition)
}

func extractNamedBlock(content, name string) (string, bool) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\{`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return "", false
	}
	openIndex := strings.Index(content[loc[0]:loc[1]], "{")
	if openIndex < 0 {
		return "", false
	}
	openIndex += loc[0]
	closeIndex := findMatchingBrace(content, openIndex)
	if closeIndex < 0 {
		return "", false
	}
	return strings.TrimSpace(content[openIndex+1 : closeIndex]), true
}

func findMatchingBrace(content string, openIndex int) int {
	depth := 0
	var quote rune
	escaped := false
	for index, char := range content {
		if index < openIndex {
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '{' {
			depth++
			continue
		}
		if char == '}' {
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func childrenOf(steps []JenkinsRenderStep, parentID string) []JenkinsRenderStep {
	children := make([]JenkinsRenderStep, 0)
	for _, item := range steps {
		if !item.Enabled {
			continue
		}
		if strings.TrimSpace(item.ParentNodeID) == parentID {
			children = append(children, item)
		}
	}
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].SortOrder < children[j].SortOrder
	})
	return children
}

func hasExplicitBranchType(children []JenkinsRenderStep) bool {
	for _, child := range children {
		if strings.TrimSpace(child.BranchType) != "" {
			return true
		}
	}
	return false
}

func splitBranchChildren(children []JenkinsRenderStep) ([]JenkinsRenderStep, []JenkinsRenderStep) {
	parallelChildren := make([]JenkinsRenderStep, 0)
	nextChildren := make([]JenkinsRenderStep, 0)
	for _, child := range children {
		if strings.TrimSpace(child.BranchType) == "parallel" {
			parallelChildren = append(parallelChildren, child)
			continue
		}
		nextChildren = append(nextChildren, child)
	}
	return parallelChildren, nextChildren
}

func parallelContinuationRoots(steps []JenkinsRenderStep, parallelChildren []JenkinsRenderStep) []JenkinsRenderStep {
	result := make([]JenkinsRenderStep, 0)
	for _, branch := range parallelChildren {
		_, nextChildren := splitBranchChildren(childrenOf(steps, branch.ID))
		result = append(result, nextChildren...)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})
	return result
}

func appendUniqueRenderSteps(base []JenkinsRenderStep, extra ...JenkinsRenderStep) []JenkinsRenderStep {
	seen := make(map[string]struct{}, len(base)+len(extra))
	result := make([]JenkinsRenderStep, 0, len(base)+len(extra))
	for _, item := range base {
		key := strings.TrimSpace(item.ID)
		if key != "" {
			seen[key] = struct{}{}
		}
		result = append(result, item)
	}
	for _, item := range extra {
		key := strings.TrimSpace(item.ID)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})
	return result
}

func stageNameFromContent(content, fallback string) string {
	match := stageNamePattern.FindStringSubmatch(content)
	if len(match) >= 3 {
		return match[2]
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "未命名步骤"
}

func stageNameForNode(step JenkinsRenderStep) string {
	if strings.TrimSpace(step.NodeName) != "" {
		return strings.TrimSpace(step.NodeName)
	}
	return stageNameFromContent(step.StageContent, step.StepType)
}

func recordStageName(stageNames map[string]string, step JenkinsRenderStep, name string) {
	id := strings.TrimSpace(step.ID)
	if id == "" || id == "workflow-start" || id == "workflow-post" {
		return
	}
	stageNames[id] = name
}

func uniqueStageName(name string, stageCount map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未命名步骤"
	}
	stageCount[name]++
	if stageCount[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, stageCount[name])
}

func indent(level int) string {
	return strings.Repeat("  ", level)
}

func indentText(text string, level int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index, line := range lines {
		lines[index] = indent(level) + strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func singleQuote(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}
