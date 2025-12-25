package verify

import (
	"reflect"
	"strconv"

	"github.com/go-playground/validator/v10"
)

// validateDefault 检查并设置默认值，只有当字段为空时才设置默认值
func validateDefault(fl validator.FieldLevel) bool {
	field := fl.Field()
	defaultValue := fl.Param()

	// 根据字段类型直接处理检查和设置默认值
	switch field.Kind() {
	case reflect.String:
		if field.String() == "" {
			field.SetString(defaultValue)
		}
	case reflect.Bool:
		if !field.Bool() {
			if newVal, err := strconv.ParseBool(defaultValue); err == nil {
				field.SetBool(newVal)
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Int() == 0 {
			if newVal, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
				field.SetInt(newVal)
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if field.Uint() == 0 {
			if newVal, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
				field.SetUint(newVal)
			}
		}
	case reflect.Float32, reflect.Float64:
		if field.Float() == 0 {
			if newVal, err := strconv.ParseFloat(defaultValue, 64); err == nil {
				field.SetFloat(newVal)
			}
		}
	default:
		// 如果类型不是上述任何一种，则不处理并返回false
		return false
	}

	return true
}

// 注册
func init() {
	var v = &ValidatorTranslation{
		Tag:            "validateDefault",
		ValidationFunc: validateDefault,
		Translations: []TranslationDetail{
			{
				Locale:          LocaleEN,
				TranslationMsg:  "{0} Set default Value",
				TranslationFunc: nil,
			},
			{
				Locale:          LocaleZH,
				TranslationMsg:  "{0} 设置默认值",
				TranslationFunc: nil,
			},
		},
	}
	RegistryValidator(v)
}
