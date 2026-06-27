package model

import (
	"testing"

	"github.com/yanshicheng/kube-nova/common/devops/channelvars"
)

func TestDefaultChannelTypesIncludeDefaultMappingFields(t *testing.T) {
	requiredFields := []string{
		channelvars.FieldChannelID,
		channelvars.FieldChannelName,
		channelvars.FieldChannelCode,
		channelvars.FieldEndpoint,
	}
	for _, item := range defaultChannelTypes {
		schema := channelvars.ParseVariableSchema(item.MappingFields)
		for _, field := range requiredFields {
			if _, ok := channelvars.FindVariableFieldInList(schema.Fields, field); !ok {
				t.Fatalf("%s/%s mapping fields should contain %s", item.GroupCode, item.Code, field)
			}
		}
	}
}

func TestDefaultStepChannelParamsAreDefined(t *testing.T) {
	if len(defaultStepChannelParams) == 0 {
		t.Fatalf("default step channel params should be defined")
	}
	seen := map[string]struct{}{}
	for _, item := range defaultStepChannelParams {
		if item.ParamType == "" {
			t.Fatalf("default step channel param type should not be empty")
		}
		if item.GroupCode == "" {
			t.Fatalf("default step channel param %s should contain group code", item.ParamType)
		}
		key := item.ParamType + "/" + item.GroupCode + "/" + item.ChannelTypeFilter
		if _, ok := seen[key]; ok {
			t.Fatalf("default step channel param duplicated: %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestDefaultChannelTypeAddressFieldsOnlyDependOnEndpoint(t *testing.T) {
	for _, item := range defaultChannelTypes {
		schema := channelvars.ParseVariableSchema(item.MappingFields)
		for _, field := range schema.Fields {
			if !channelvars.IsAddressVariableField(field) {
				continue
			}
			if len(channelvars.RequiredParamDependencies(field)) != 0 {
				t.Fatalf("%s/%s address field %s should not expose param dependencies", item.GroupCode, item.Code, field.Field)
			}
			if len(channelvars.DependencyAllOf(field)) != 0 {
				t.Fatalf("%s/%s address field %s should not require external params", item.GroupCode, item.Code, field.Field)
			}
		}
	}
}
