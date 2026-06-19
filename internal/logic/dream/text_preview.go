package dream

func previewRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
