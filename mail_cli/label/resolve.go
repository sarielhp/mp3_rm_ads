package label

import "strings"

func ResolveFolder(folders []LabelItem, name string, fallback string) string {
	if name != "" {
		for _, f := range folders {
			if strings.EqualFold(f.FullName, name) {
				return f.FullName
			}
		}
		lower := strings.ToLower(name)
		for _, f := range folders {
			if strings.Contains(strings.ToLower(f.FullName), lower) {
				return f.FullName
			}
		}
	}
	if fallback != "" {
		for _, f := range folders {
			if strings.EqualFold(f.FullName, fallback) {
				return f.FullName
			}
		}
	}
	for _, f := range folders {
		if strings.EqualFold(f.FullName, "INBOX") {
			return f.FullName
		}
	}
	if len(folders) > 0 {
		return folders[0].FullName
	}
	return name
}
