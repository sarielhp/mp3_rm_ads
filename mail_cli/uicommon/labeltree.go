package uicommon

import (
	"fmt"
	"mail_cli/app"
	"mail_cli/label"

	"github.com/fatih/color"
)

// BuildLabelTree delegates to label.BuildLabelTree.
func BuildLabelTree(items []label.LabelItem) []*label.LabelTreeNode {
	return label.BuildLabelTree(items)
}

// FlattenTree delegates to label.FlattenTree.
func FlattenTree(nodes []*label.LabelTreeNode) []label.TreeEntry {
	return label.FlattenTree(nodes)
}

// CountAllDescendants delegates to label.CountAllDescendants.
func CountAllDescendants(n *label.LabelTreeNode) int {
	return label.CountAllDescendants(n)
}

func PrintLabelTree(headerTitle string, items []label.LabelItem, hideZero bool) {
	fmt.Println()
	app.ColorBoldCyan.Println("======================================================================")
	app.ColorBoldCyan.Println(headerTitle)
	app.ColorBoldCyan.Println("======================================================================")

	if len(items) == 0 {
		fmt.Println("  No labels/folders found.")
		return
	}

	tree := label.BuildLabelTree(items)

	var printNode func([]*label.LabelTreeNode, string, int, bool)
	printNode = func(nodes []*label.LabelTreeNode, prefix string, depth int, hideZero bool) {
		var visible []*label.LabelTreeNode
		for _, node := range nodes {
			if hideZero && node.MessagesTotal == 0 {
				continue
			}
			visible = append(visible, node)
		}

		colors := []*color.Color{
			app.ColorBoldCyan,
			app.ColorBoldGreen,
			app.ColorBoldYellow,
			app.ColorBoldPurple,
			app.ColorWhite,
		}
		depthColor := colors[depth%len(colors)]

		for i, node := range visible {
			isLast := i == len(visible)-1
			connector := "├── "
			if isLast {
				connector = "└── "
			}

			nameStyled := depthColor.Sprint(node.Name)

			if node.IsLabel {
				if node.MessagesUnread > 0 {
					fmt.Printf("%s%s%s (%d/%d)\n", prefix, connector, nameStyled, node.MessagesUnread, node.MessagesTotal)
				} else {
					fmt.Printf("%s%s%s (%d)\n", prefix, connector, nameStyled, node.MessagesTotal)
				}
			} else {
				fmt.Printf("%s%s%s [Folder]\n", prefix, connector, nameStyled)
			}

			if len(node.Children) > 0 {
				nextPrefix := prefix + "│   "
				if isLast {
					nextPrefix = prefix + "    "
				}
				printNode(node.Children, nextPrefix, depth+1, hideZero)
			}
		}
	}

	printNode(tree, "", 0, hideZero)
}
