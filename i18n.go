package main

import "strings"

// lang 是界面语言："zh" 或 "en"。默认跟随系统的显示语言（uiLocale），可在设置里改（config.json 的 language）。
var lang = "en"

// initLang 按设置定界面语言；改了设置后再调一次即生效，跟随系统时先回到系统的语言。
func initLang(cfg *config) { lang = pickLang(cfg.Language, uiLocale()) }

// pickLang 是设置（"zh"、"en"；其他为跟随系统）与系统语言（如 "zh-Hans-CN"）定下的界面语言。
func pickLang(setting, locale string) string {
	switch {
	case setting == "zh" || setting == "en":
		return setting
	case strings.HasPrefix(locale, "zh"):
		return "zh"
	}
	return "en"
}

// tr 返回当前语言的文案；格式化文案的两种语言须用相同的占位符（可用 %[2]s 这类带序号的）。
func tr(zh, en string) string {
	if lang == "zh" {
		return zh
	}
	return en
}
