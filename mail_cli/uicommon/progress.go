package uicommon

import (
	"fmt"
	"os"
	"strings"
)

func DrawProgressBar(current, total int, prefix string) {
	if total <= 0 {
		return
	}
	width := 30
	percent := float64(current) / float64(total)
	filledLength := int(float64(width) * percent)

	var sb strings.Builder
	sb.WriteString("\r" + prefix + " [")
	for i := 0; i < filledLength; i++ {
		sb.WriteString("█")
	}
	for i := filledLength; i < width; i++ {
		sb.WriteString("░")
	}
	sb.WriteString(fmt.Sprintf("] %.0f%% (%d/%d)", percent*100, current, total))
	if current == total {
		sb.WriteString("\n")
	}
	fmt.Print(sb.String())
	os.Stdout.Sync()
}
