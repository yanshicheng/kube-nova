package verify

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validateInstance *validator.Validate
	translator       ut.Translator
)

// ValidatorInstance 包含 Validator 和 Translator 实例
type ValidatorInstance struct {
	Validate   *validator.Validate
	Translator ut.Translator
}

// InitValidator 初始化验证器和翻译系统
func InitValidator(language LocaleType) (*ValidatorInstance, error) {
	// 创建 validator 实例
	v := validator.New()

	// 注册 JSON 标签解析，显示友好的错误信息
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// 初始化翻译器
	zhT := zh.New() // 中文翻译器
	enT := en.New() // 英文翻译器
	uni := ut.New(enT, zhT, enT)

	// 根据语言选择翻译器
	var trans ut.Translator
	var ok bool
	switch language {
	case "zh":
		trans, ok = uni.GetTranslator("zh")
		if !ok {
			return nil, fmt.Errorf("找不到对应语言的翻译器: %s", "zh")
		}
		if err := zhTranslations.RegisterDefaultTranslations(v, trans); err != nil {
			return nil, err
		}
	case "en":
		trans, ok = uni.GetTranslator("en")
		if !ok {
			return nil, fmt.Errorf("找不到对应语言的翻译器: %s", "en")
		}
		if err := enTranslations.RegisterDefaultTranslations(v, trans); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("不支持的语言: %s", language)
	}

	return &ValidatorInstance{
		Validate:   v,
		Translator: trans,
	}, nil
}

// RemoveTopStruct 移除验证错误信息中的结构体名称前缀
func RemoveTopAsStruct(err validator.ValidationErrors, trans ut.Translator) map[string]string {
	res := map[string]string{}
	for field, message := range err.Translate(trans) {
		// 去掉字段前的结构体名称，保留具体字段名
		res[field[strings.Index(field, ".")+1:]] = message
	}
	return res
}

func RemoveTopSaStr(err validator.ValidationErrors, trans ut.Translator) string {
	mapError := RemoveTopAsStruct(err, trans)
	return MapToString(mapError)
}

// MapToString 将 map 转换为字符串，并以 "!" 结尾
func MapToString(data map[string]string) string {
	var sb strings.Builder
	for key, value := range data {
		sb.WriteString(fmt.Sprintf("%s: %s, ", key, value))
	}
	result := sb.String()
	if len(result) > 0 {
		result = result[:len(result)-2] + "!" // 去掉最后的逗号和空格，替换为 "!"
	}
	return result
}
