package utils

// GetLanguageName returns the full language name for the given language code
func GetLanguageName(language string) string {
	switch language {
	case "zh":
		return "Chinese"
	case "en":
		return "English"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	case "fr":
		return "French"
	case "de":
		return "German"
	case "es":
		return "Spanish"
	case "ru":
		return "Russian"
	default:
		return "English"
	}
}
