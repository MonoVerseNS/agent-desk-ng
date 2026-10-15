package ai

import (
	"unicode"
)

// DetectLanguage inspects a text and returns the predominant language: "ru", "zh", or "en".
func DetectLanguage(text string) string {
	cyrillicCount := 0
	hanCount := 0
	latinCount := 0

	for _, r := range text {
		if unicode.Is(unicode.Cyrillic, r) {
			cyrillicCount++
		} else if unicode.Is(unicode.Han, r) {
			hanCount++
		} else if unicode.Is(unicode.Latin, r) {
			latinCount++
		}
	}

	if cyrillicCount > 0 && cyrillicCount >= hanCount {
		return "ru"
	}
	if hanCount > 0 && hanCount > cyrillicCount {
		return "zh"
	}
	if latinCount > 0 {
		return "en"
	}
	return "en"
}

// IsCyrillic returns true if the string contains at least one Cyrillic rune.
func IsCyrillic(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// IsHan returns true if the string contains at least one Han (Chinese) rune.
func IsHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
