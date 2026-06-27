package pipeline

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
)

func TestUpdateDevopsPipelineRequestEngineTypeDoesNotDefaultToJenkins(t *testing.T) {
	field, ok := reflect.TypeOf(types.UpdateDevopsPipelineRequest{}).FieldByName("EngineType")
	if !ok {
		t.Fatal("EngineType 字段不存在")
	}
	tag := string(field.Tag)
	if strings.Contains(tag, `default:"jenkins"`) {
		t.Fatalf("更新流水线不能默认 engineType=jenkins: %s", tag)
	}
	if !strings.Contains(tag, "omitempty,oneof=jenkins tekton") {
		t.Fatalf("engineType 更新校验必须允许空值并限制枚举: %s", tag)
	}
}
