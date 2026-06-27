package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRenderDeclarativePipelineSupportsNestedParallelByScriptedFallback(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "step-1",
				NodeName:     "步骤一",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "stage('步骤一') { steps { echo '1' } }",
			},
			{
				ID:           "step-2",
				NodeName:     "步骤二",
				ParentNodeID: "workflow-start",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "stage('步骤二') { steps { echo '2' } }",
			},
			{
				ID:           "step-3",
				NodeName:     "步骤三",
				ParentNodeID: "step-1",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "stage('步骤三') { steps { echo '3' } }",
			},
			{
				ID:           "step-4",
				NodeName:     "步骤四",
				ParentNodeID: "step-1",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "stage('步骤四') { steps { echo '4' } }",
			},
		},
	})

	if err != nil {
		t.Fatalf("nested parallel should be rendered: %v", err)
	}
	if !containsAll(pipeline, "stage('并行组-1')", "script {", "parallel(") {
		t.Fatalf("pipeline does not include scripted nested parallel fallback:\n%s", pipeline)
	}
	if contains(pipeline, "parallel {\n") {
		t.Fatalf("nested parallel fallback should not use declarative parallel:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineWrapsStepsBody(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "step-1",
				NodeName:     "信息展示",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "script {\n  echo 'hello'\n}",
			},
			{
				ID:           "step-2",
				NodeName:     "测试",
				ParentNodeID: "workflow-start",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "echo 'test'",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !containsAll(pipeline, "stage('并行组-1')", "parallel {", "stage('信息展示')", "steps {") {
		t.Fatalf("pipeline does not include expected declarative structure:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineKeepsNextStageAfterExplicitParallel(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "clone",
				NodeName:     "代码克隆",
				ParentNodeID: "workflow-start",
				BranchType:   "next",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps { echo 'clone' }",
			},
			{
				ID:           "branch-a",
				NodeName:     "分支A",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "steps { echo 'a' }",
			},
			{
				ID:           "branch-b",
				NodeName:     "分支B",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    3,
				Enabled:      true,
				StageContent: "steps { echo 'b' }",
			},
			{
				ID:           "deploy",
				NodeName:     "部署",
				ParentNodeID: "clone",
				BranchType:   "next",
				SortOrder:    4,
				Enabled:      true,
				StageContent: "steps { echo 'deploy' }",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	parallelIndex := indexOf(pipeline, "parallel {")
	deployIndex := indexOf(pipeline, "stage('部署')")
	if parallelIndex < 0 || deployIndex < 0 {
		t.Fatalf("pipeline should contain parallel group and next stage:\n%s", pipeline)
	}
	if deployIndex < parallelIndex {
		t.Fatalf("next stage should be rendered after explicit parallel group:\n%s", pipeline)
	}
	parallelEndIndex := indexOf(pipeline, "    }\n    stage('部署')")
	if parallelEndIndex < 0 || parallelEndIndex < parallelIndex || parallelEndIndex > deployIndex {
		t.Fatalf("next stage should not be rendered inside the parallel block:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineMovesParallelBranchNextChainAfterJoin(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "clone",
				NodeName:     "Git代码克隆",
				ParentNodeID: "workflow-start",
				BranchType:   "next",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps { echo 'clone' }",
			},
			{
				ID:           "sonar",
				NodeName:     "Sonar扫描",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "steps { echo 'sonar' }",
			},
			{
				ID:           "npm",
				NodeName:     "npm编译",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    3,
				Enabled:      true,
				StageContent: "steps { echo 'npm' }",
			},
			{
				ID:           "custom",
				NodeName:     "自定义命令",
				ParentNodeID: "npm",
				BranchType:   "next",
				SortOrder:    4,
				Enabled:      true,
				StageContent: "steps { echo 'custom' }",
			},
			{
				ID:           "docker",
				NodeName:     "docker构建",
				ParentNodeID: "custom",
				BranchType:   "next",
				SortOrder:    5,
				Enabled:      true,
				StageContent: "steps { echo 'docker' }",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if contains(pipeline, "stage('npm编译-分支')") {
		t.Fatalf("parallel branch next chain should not be rendered inside branch stages:\n%s", pipeline)
	}
	parallelIndex := indexOf(pipeline, "parallel {")
	customIndex := indexOf(pipeline, "stage('自定义命令')")
	dockerIndex := indexOf(pipeline, "stage('docker构建')")
	if parallelIndex < 0 || customIndex < 0 || dockerIndex < 0 {
		t.Fatalf("pipeline should contain parallel group and joined next stages:\n%s", pipeline)
	}
	if customIndex < parallelIndex || dockerIndex < customIndex {
		t.Fatalf("next chain should render after the parallel group in order:\n%s", pipeline)
	}
	if !contains(pipeline, "    }\n    stage('自定义命令')") {
		t.Fatalf("custom stage should be outside the parallel block:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineSupportsSequentialParallelGroups(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "clone",
				NodeName:     "拉取代码",
				ParentNodeID: "workflow-start",
				BranchType:   "next",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps { echo 'clone' }",
			},
			{
				ID:           "compile",
				NodeName:     "编译",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "steps { echo 'compile' }",
			},
			{
				ID:           "scan",
				NodeName:     "扫描",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    3,
				Enabled:      true,
				StageContent: "steps { echo 'scan' }",
			},
			{
				ID:           "build",
				NodeName:     "构建",
				ParentNodeID: "clone",
				BranchType:   "next",
				SortOrder:    4,
				Enabled:      true,
				StageContent: "steps { echo 'build' }",
			},
			{
				ID:           "package",
				NodeName:     "打包镜像",
				ParentNodeID: "build",
				BranchType:   "next",
				SortOrder:    5,
				Enabled:      true,
				StageContent: "steps { echo 'package' }",
			},
			{
				ID:           "push",
				NodeName:     "上传镜像",
				ParentNodeID: "package",
				BranchType:   "parallel",
				SortOrder:    6,
				Enabled:      true,
				StageContent: "steps { echo 'push' }",
			},
			{
				ID:           "image-scan",
				NodeName:     "扫描镜像",
				ParentNodeID: "package",
				BranchType:   "parallel",
				SortOrder:    7,
				Enabled:      true,
				StageContent: "steps { echo 'image-scan' }",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	order := []string{
		"stage('拉取代码')",
		"stage('并行组-1')",
		"stage('编译')",
		"stage('扫描')",
		"stage('构建')",
		"stage('打包镜像')",
		"stage('并行组-2')",
		"stage('上传镜像')",
		"stage('扫描镜像')",
	}
	last := -1
	for _, item := range order {
		index := indexOf(pipeline, item)
		if index < 0 {
			t.Fatalf("pipeline should contain %s:\n%s", item, pipeline)
		}
		if index < last {
			t.Fatalf("pipeline stage %s rendered out of order:\n%s", item, pipeline)
		}
		last = index
	}
	if contains(pipeline, "stage('编译-分支')") || contains(pipeline, "stage('上传镜像-分支')") {
		t.Fatalf("simple parallel branches should render directly under parallel block:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineKeepsPostOutsideSequentialParallelGroups(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "clone",
				NodeName:     "拉取代码",
				ParentNodeID: "workflow-start",
				BranchType:   "next",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps { echo 'clone' }",
			},
			{
				ID:           "compile",
				NodeName:     "编译",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "steps { echo 'compile' }",
			},
			{
				ID:           "scan",
				NodeName:     "扫描",
				ParentNodeID: "clone",
				BranchType:   "parallel",
				SortOrder:    3,
				Enabled:      true,
				StageContent: "steps { echo 'scan' }",
			},
			{
				ID:           "build",
				NodeName:     "构建",
				ParentNodeID: "clone",
				BranchType:   "next",
				SortOrder:    4,
				Enabled:      true,
				StageContent: "steps { echo 'build' }",
			},
			{
				ID:           "notify",
				NodeName:     "always",
				ParentNodeID: "workflow-post",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "always { echo 'cleanup' }",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	stagesIndex := indexOf(pipeline, "stages {")
	postIndex := indexOf(pipeline, "post {")
	if stagesIndex < 0 || postIndex < 0 || postIndex < stagesIndex {
		t.Fatalf("post block should render after stages:\n%s", pipeline)
	}
	if !containsAll(pipeline, "stage('并行组-1')", "stage('构建')", "post {", "always {", "echo 'cleanup'") {
		t.Fatalf("pipeline should contain parallel group, next stage and post block:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineRejectsDuplicatePostCondition(t *testing.T) {
	_, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "notify",
				NodeName:     "成功通知",
				ParentNodeID: "workflow-post",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "success { echo 'ok' }",
			},
			{
				ID:           "archive",
				NodeName:     "成功归档",
				ParentNodeID: "workflow-post",
				SortOrder:    2,
				Enabled:      true,
				StageContent: "success { echo 'again' }",
			},
		},
	})
	if err == nil || !contains(err.Error(), "post 条件 success 只能配置一次") {
		t.Fatalf("duplicate success post condition should be rejected, got %v", err)
	}
}

func TestRenderDeclarativePipelineKeepsMultiplePostConditionsInOneStep(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "notify",
				NodeName:     "通知",
				ParentNodeID: "workflow-post",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "success { echo 'success' }\nfailure { echo 'failure' }",
			},
		},
	})
	if err != nil {
		t.Fatalf("multiple post conditions in one step should be rendered: %v", err)
	}
	if !containsAll(pipeline, "post {", "success {", "echo 'success'", "failure {", "echo 'failure'") {
		t.Fatalf("pipeline should keep all post conditions:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineGuardsEmptyPostShellScript(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "failure-script",
				NodeName:     "失败脚本",
				ParentNodeID: "workflow-post",
				SortOrder:    1,
				Enabled:      true,
				StageContent: `failure {
  script {
    def scriptContent = params.POST_FAILURE_SCRIPT
    sh scriptContent
  }
}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !containsAll(pipeline, "if (scriptContent?.trim())", "sh scriptContent", "post 脚本为空，跳过") {
		t.Fatalf("post shell script should be guarded:\n%s", pipeline)
	}
}

func TestPipelineJobXMLRemovesInvalidXMLCharacters(t *testing.T) {
	xml := pipelineJobXML("测试\x8b流水线", "echo 'ok'\x8b")
	if hasInvalidXMLRune(xml) {
		t.Fatalf("config xml should remove invalid XML control characters: %q", xml)
	}
	if !containsAll(xml, "测试流水线", "echo &apos;ok&apos;") {
		t.Fatalf("config xml lost valid content:\n%s", xml)
	}
}

func TestPipelineJobXMLIncludesJobParameters(t *testing.T) {
	xml := pipelineJobXMLWithParams("测试流水线", "echo 'ok'", []JenkinsRenderParam{
		{Code: "VERSION", ParamType: "string", Mode: "params", CurrentValue: "1.0.0", Description: "版本"},
		{Code: "TIMEOUT_MINUTES", ParamType: "number", Mode: "params", CurrentValue: "30", Description: "超时时间"},
		{Code: "CONFIRM", ParamType: "booleanParam", Mode: "params", CurrentValue: "true", Description: "确认"},
		{Code: "SECRET", ParamType: "password", Mode: "params", Description: "密钥"},
		{Code: "APP_NAME", ParamType: "string", Mode: "env", CurrentValue: "demo"},
	})
	if !containsAll(xml,
		"<hudson.model.ParametersDefinitionProperty>",
		"<hudson.model.StringParameterDefinition>",
		"<name>VERSION</name>",
		"<name>TIMEOUT_MINUTES</name>",
		"<hudson.model.BooleanParameterDefinition>",
		"<name>CONFIRM</name>",
		"<hudson.model.PasswordParameterDefinition>",
		"<name>SECRET</name>",
	) {
		t.Fatalf("config xml should include Jenkins job parameters:\n%s", xml)
	}
	if contains(xml, "<name>APP_NAME</name>") {
		t.Fatalf("env variables should not be written as Jenkins build parameters:\n%s", xml)
	}
}

func TestRenderDeclarativePipelineNumberParamUsesStringCarrier(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Params: []JenkinsRenderParam{
			{Code: "BASH_TIMEOUT_MINUTES", ParamType: "number", Mode: "params", CurrentValue: "30", Description: "命令执行生命周期"},
		},
		Steps: []JenkinsRenderStep{
			{
				ID:           "step-1",
				NodeName:     "执行命令",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps {\n  script {\n    echo params.BASH_TIMEOUT_MINUTES\n  }\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !contains(pipeline, "string(name: 'BASH_TIMEOUT_MINUTES', defaultValue: '30'") {
		t.Fatalf("number param should use Jenkins string carrier:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineTextParamUsesTripleSingleQuotes(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Params: []JenkinsRenderParam{
			{Code: "BASH_COMMANDS", ParamType: "text", Mode: "params", CurrentValue: "ls\npwd\ndocker -v", Description: "执行命令，一行一个"},
		},
		Steps: []JenkinsRenderStep{
			{
				ID:           "step-1",
				NodeName:     "执行命令",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "steps {\n  script {\n    echo params.BASH_COMMANDS\n  }\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !contains(pipeline, "text(name: 'BASH_COMMANDS', defaultValue: '''ls\npwd\ndocker -v''', description: '执行命令，一行一个')") {
		t.Fatalf("text param should use triple single quotes:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineWritesMultilineEnvAsTripleQuotedString(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Environment: map[string]string{
			"REPOS_JSON": "[\n  {\"repoName\":\"repo-a\"}\n]",
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !contains(pipeline, "REPOS_JSON = '''[\n  {\"repoName\":\"repo-a\"}\n]'''") {
		t.Fatalf("multiline env should use triple single quotes:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineNormalizesPasswordLengthUsage(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Params: []JenkinsRenderParam{
			{Code: "API_TOKEN", ParamType: "password", Mode: "params"},
		},
		Steps: []JenkinsRenderStep{
			{
				ID:           "step-1",
				NodeName:     "信息展示",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "script {\n  def tokenLen = params.API_TOKEN ? params.API_TOKEN.length() : 0\n  echo \"${tokenLen}\"\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if contains(pipeline, "params.API_TOKEN.length()") {
		t.Fatalf("password parameter length should be converted to string length:\n%s", pipeline)
	}
	if !contains(pipeline, "params.API_TOKEN ? params.API_TOKEN.toString().length() : 0") {
		t.Fatalf("pipeline should contain Jenkins Secret compatible password length:\n%s", pipeline)
	}
}

func TestRenderDeclarativePipelineUsesNodeNameAndReturnsStageMap(t *testing.T) {
	result, err := RenderDeclarativePipelineWithMetadata(RenderPipeline{
		Steps: []JenkinsRenderStep{
			{
				ID:           "node-1",
				NodeName:     "卡片步骤",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "stage('旧模板名称') { steps { echo '1' } }",
			},
			{
				ID:           "node-2",
				NodeName:     "卡片步骤",
				ParentNodeID: "node-1",
				SortOrder:    1,
				Enabled:      true,
				StageContent: "stage('旧模板名称') { steps { echo '2' } }",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if contains(result.Pipeline, "stage('旧模板名称')") {
		t.Fatalf("pipeline should use workflow node name instead of template stage name:\n%s", result.Pipeline)
	}
	if !containsAll(result.Pipeline, "stage('卡片步骤')", "stage('卡片步骤-2')") {
		t.Fatalf("pipeline should contain unique workflow node stage names:\n%s", result.Pipeline)
	}
	if result.StageNames["node-1"] != "卡片步骤" || result.StageNames["node-2"] != "卡片步骤-2" {
		t.Fatalf("stage map mismatch: %#v", result.StageNames)
	}
}

func TestRenderDeclarativePipelineRewritesStepParamReferences(t *testing.T) {
	pipeline, err := RenderDeclarativePipeline(RenderPipeline{
		Params: []JenkinsRenderParam{
			{Code: "BASH_COMMANDS_2", ParamType: "text", Mode: "params"},
			{Code: "BASH_TIMEOUT_MINUTES_2", ParamType: "number", Mode: "params"},
		},
		Steps: []JenkinsRenderStep{
			{
				ID:           "node-2",
				NodeName:     "自定义脚本-2",
				ParentNodeID: "workflow-start",
				SortOrder:    1,
				Enabled:      true,
				ParamCodeMap: map[string]string{
					"BASH_COMMANDS":        "BASH_COMMANDS_2",
					"BASH_TIMEOUT_MINUTES": "BASH_TIMEOUT_MINUTES_2",
				},
				StageContent: "steps {\n  script {\n    echo params.BASH_COMMANDS\n    echo params['BASH_COMMANDS']\n    echo params[\"BASH_COMMANDS\"]\n    echo env.BASH_COMMANDS\n    echo \"${BASH_COMMANDS}\"\n    echo \"$BASH_COMMANDS\"\n    echo params.BASH_TIMEOUT_MINUTES as Integer\n  }\n}",
			},
		},
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if contains(pipeline, "params.BASH_COMMANDS\n") || contains(pipeline, "params['BASH_COMMANDS']") || contains(pipeline, "params[\"BASH_COMMANDS\"]") || contains(pipeline, "env.BASH_COMMANDS\n") {
		t.Fatalf("source param references should be rewritten:\n%s", pipeline)
	}
	if !containsAll(pipeline, "params.BASH_COMMANDS_2", "params['BASH_COMMANDS_2']", "env.BASH_COMMANDS_2", "${BASH_COMMANDS_2}", "$BASH_COMMANDS_2", "params.BASH_TIMEOUT_MINUTES_2 as Integer") {
		t.Fatalf("pipeline should contain rewritten references:\n%s", pipeline)
	}
}

func TestCleanJenkinsErrorBodySkipsGenericOops(t *testing.T) {
	body := `<html><head><title>Jenkins - Jenkins</title></head><body><h1>Oops!</h1><pre>java.io.IOException: Failed to persist config.xml</pre></body></html>`
	message := cleanJenkinsErrorBody(body)
	if !contains(message, "Failed to persist config.xml") {
		t.Fatalf("should prefer useful Jenkins error text, got: %s", message)
	}
}

func TestCleanJenkinsLogTextRemovesHTMLTimestamp(t *testing.T) {
	raw := `[Print Message]
<span class="timestamp"><b>23:23:36</b> </span><span style="display: none">[2026-05-03T15:23:36.275Z]</span> hello`
	text := cleanJenkinsLogText(raw)
	if contains(text, "<span") || contains(text, "2026-05-03T") || contains(text, "[Print Message]") {
		t.Fatalf("log should remove Jenkins html noise: %q", text)
	}
	if !contains(text, "23:23:36 hello") {
		t.Fatalf("log should keep readable timestamp and message: %q", text)
	}
}

func TestCleanJenkinsLogTextRemovesEscapedHTMLTimestamp(t *testing.T) {
	raw := `[Print Message]
&lt;span class=&#34;timestamp&#34;&gt;&lt;b&gt;23:23:36&lt;/b&gt; &lt;/span&gt;&lt;span style=&#34;display: none&#34;&gt;[2026-05-03T15:23:36.275Z]&lt;/span&gt; hello`
	text := cleanJenkinsLogText(raw)
	if contains(text, "<span") || contains(text, "2026-05-03T") || contains(text, "[Print Message]") {
		t.Fatalf("log should remove escaped Jenkins html noise: %q", text)
	}
	if !contains(text, "23:23:36 hello") {
		t.Fatalf("log should keep readable timestamp and message: %q", text)
	}
}

func TestCleanJenkinsLogTextRemovesConsoleNotesAndAnsi(t *testing.T) {
	raw := "Started by user \x1b[8mha:////4FySlCRW34BMWcMx5VSNzHFcDFvoTjk3AAA=\x1b[0mJenkins Admin\n" +
		"\x1b[32m[Pipeline] echo\x1b[0m\n" +
		"[2026-05-04T13:55:23.949Z] 输出数字: 1\n"
	text := cleanJenkinsLogText(raw)
	if contains(text, "ha:////") || contains(text, "\x1b") {
		t.Fatalf("log should remove Jenkins console notes and ansi codes: %q", text)
	}
	if !containsAll(text, "Started by user Jenkins Admin", "[Pipeline] echo", "输出数字: 1") {
		t.Fatalf("log should keep readable content: %q", text)
	}
}

func TestCleanJenkinsLogTextRemovesUnclosedConsoleNote(t *testing.T) {
	raw := "Started by user \x1b[8mha:////4FySlCRW34BMWcMx5VSNzHFcDFvoTjk3aXjrNKWGl902AAAAlx+LCAAAAAAAAP9b85aBtbiIQTGjNKU4P08vOT+vOD8nVc83PyU1x6OyILUoJzMv2y+/JJUBAhiZGBgqihhk0NSjKDWzXb3RdlLBUSYGJk8GtpzUvPSSDB8G5tKinBIGIZ+sxLJE/ZzEvHT94JKizLx0a6BxUmjGOUNodHsLgAzeEgZu/dLi1CL9xJTczDwACG0V4sAAAAA=\x1b[0mJenkins Admin\nFinished: SUCCESS\n"
	text := cleanJenkinsLogText(raw)
	if contains(text, "ha:////") || contains(text, "\x1b") || contains(text, "LCAAAAA") {
		t.Fatalf("log should remove long Jenkins console note: %q", text)
	}
	if !containsAll(text, "Started by user Jenkins Admin", "Finished: SUCCESS") {
		t.Fatalf("log should keep readable content: %q", text)
	}
}

func TestCleanJenkinsLogTextRemovesBareConsoleNote(t *testing.T) {
	raw := "Running on ha:////4LSwW2+XiqgapeJqbtP6UwCiWB/zRHMKAAAA Jenkins in /workspace/demo\n"
	text := cleanJenkinsLogText(raw)
	if contains(text, "ha:////") {
		t.Fatalf("log should remove bare Jenkins console note: %q", text)
	}
	if !contains(text, "Running on Jenkins in /workspace/demo") {
		t.Fatalf("log should keep readable content: %q", text)
	}
}

func TestBuildWithParametersUsesPlainBuildWhenNoParams(t *testing.T) {
	calledPath := ""
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crumbIssuer/api/json" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"crumbRequestField": "Jenkins-Crumb",
				"crumb":             "test-crumb",
			})
			return
		}
		calledPath = r.URL.Path
		w.Header().Set("Location", server.URL+"/queue/item/101/")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	result, err := manager.BuildWithParameters(context.Background(), "demo", map[string]string{})
	if err != nil {
		t.Fatalf("plain build should succeed: %v", err)
	}
	if calledPath != "/job/demo/build" {
		t.Fatalf("empty params should call plain build, got path %s", calledPath)
	}
	if result.QueueID != "101" {
		t.Fatalf("queue id mismatch: %#v", result)
	}
	if result.BuildURL != "" {
		t.Fatalf("build url should stay empty before queue resolves: %#v", result)
	}
}

func TestBuildInfoUsesExternalEndpointURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/demo/4/api/json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number":   4,
			"building": false,
			"result":   "SUCCESS",
			"url":      "http://jenkins:8080/job/demo/4/",
		})
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	info, err := manager.BuildInfo(context.Background(), "demo", 4)
	if err != nil {
		t.Fatalf("build info should succeed: %v", err)
	}
	if info.Url != server.URL+"/job/demo/4/" {
		t.Fatalf("build url should use external endpoint, got %s", info.Url)
	}
}

func TestQueueBuildUsesExternalEndpointURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queue/item/42/api/json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 42,
			"executable": map[string]any{
				"number": 4,
				"url":    "http://jenkins:8080/job/demo/4/",
			},
		})
	}))
	defer server.Close()

	manager := NewManager(ClientConfig{Endpoint: server.URL})
	buildNumber, buildURL, ready, err := manager.QueueBuild(context.Background(), "42")
	if err != nil {
		t.Fatalf("queue build should succeed: %v", err)
	}
	if !ready || buildNumber != 4 {
		t.Fatalf("queue build result mismatch: ready=%v buildNumber=%d", ready, buildNumber)
	}
	if buildURL != server.URL+"/job/demo/4/" {
		t.Fatalf("queue build url should use external endpoint, got %s", buildURL)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !contains(value, part) {
			return false
		}
	}
	return true
}

func contains(value, part string) bool {
	for idx := 0; idx+len(part) <= len(value); idx++ {
		if value[idx:idx+len(part)] == part {
			return true
		}
	}
	return false
}

func indexOf(value, part string) int {
	for idx := 0; idx+len(part) <= len(value); idx++ {
		if value[idx:idx+len(part)] == part {
			return idx
		}
	}
	return -1
}

func hasInvalidXMLRune(value string) bool {
	for _, r := range value {
		switch r {
		case '\t', '\n', '\r':
			continue
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return true
		}
	}
	return false
}
