package verify

import (
	"fmt"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// ValidatorSlice 定义了 ValidatorTranslation 的切片
var ValidatorSlice []*ValidatorTranslation

// LocaleType 定义了一种新的类型，用于限定 Locale 的值
type LocaleType string

// 定义 LocaleType 可以接受的常量值
const (
	LocaleEN LocaleType = "en"
	LocaleZH LocaleType = "zh"
)

// ValidatorTranslation 自定义验证器和翻译的结构定义
type ValidatorTranslation struct {
	Tag            string              // 验证器标签
	ValidationFunc validator.Func      // 验证函数
	Translations   []TranslationDetail // 翻译详情列表
}

type TranslationDetail struct {
	Locale          LocaleType                // 语言环境，如'en'或'zh'
	TranslationMsg  string                    // 翻译文本
	TranslationFunc validator.TranslationFunc // 翻译函数
}

// RegisterValidatorsAndTranslations 注册验证器和翻译
func RegisterValidatorsAndTranslations(validators []*ValidatorTranslation, v *validator.Validate, uni *ut.UniversalTranslator) error {
	transEn, _ := uni.GetTranslator(string(LocaleEN))
	transZh, _ := uni.GetTranslator(string(LocaleZH))

	for _, vt := range validators {
		// 注册自定义验证器
		if err := v.RegisterValidation(vt.Tag, vt.ValidationFunc); err != nil {
			return fmt.Errorf("注册验证器失败 [%s]: %v", vt.Tag, err)
		}

		// 为每个语言环境注册翻译
		for _, td := range vt.Translations {
			var trans ut.Translator
			switch td.Locale {
			case LocaleEN:
				trans = transEn
			case LocaleZH:
				trans = transZh
			default:
				continue
			}

			// 使用默认翻译函数
			if td.TranslationFunc == nil {
				td.TranslationFunc = defaultTranslationFunc
			}

			// 注册翻译
			err := v.RegisterTranslation(vt.Tag, trans, func(ut ut.Translator) error {
				return ut.Add(vt.Tag, td.TranslationMsg, true)
			}, td.TranslationFunc)

			if err != nil {
				return fmt.Errorf("注册翻译失败 [%s]: %v", vt.Tag, err)
			}
		}
	}
	return nil
}

// defaultTranslationFunc 默认的翻译函数
func defaultTranslationFunc(ut ut.Translator, fe validator.FieldError) string {
	t, _ := ut.T(fe.Tag(), fe.Field())
	return t
}

// RegistryValidator 添加验证器到全局切片
func RegistryValidator(vt *ValidatorTranslation) {
	ValidatorSlice = append(ValidatorSlice, vt)
}
