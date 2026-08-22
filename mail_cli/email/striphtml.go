package email

import (
	"regexp"
	"strings"
)

var htmlTagRegex = regexp.MustCompile(`<[^>]+>|<![^>]*>`)
var htmlBlockRegex = regexp.MustCompile(`(?i)<script[^>]*>[\s\S]*?</script>|<style[^>]*>[\s\S]*?</style>|<title[^>]*>[\s\S]*?</title>|<head[^>]*>[\s\S]*?</head>`)

func StripHTML(text string) string {
	clean := htmlBlockRegex.ReplaceAllString(text, " ")
	clean = htmlTagRegex.ReplaceAllString(clean, "")
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = strings.ReplaceAll(clean, "&lt;", "<")
	clean = strings.ReplaceAll(clean, "&gt;", ">")
	clean = strings.ReplaceAll(clean, "&amp;", "&")
	clean = strings.ReplaceAll(clean, "&quot;", "\"")
	clean = strings.ReplaceAll(clean, "&#39;", "'")
	words := strings.Fields(clean)
	return strings.Join(words, " ")
}
