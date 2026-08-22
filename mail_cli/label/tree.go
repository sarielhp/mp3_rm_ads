package label

import (
	"sort"
	"strings"
)

func BuildLabelTree(items []LabelItem) []*LabelTreeNode {
	// Sort items once at the top level
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].FullName) < strings.ToLower(items[j].FullName)
	})

	root := &LabelTreeNode{Name: "root"}

	// Build tree in a single pass
	for _, item := range items {
		parts := strings.Split(item.FullName, "/")
		current := root
		for i, part := range parts {
			var found *LabelTreeNode
			// Use binary search for sorted children
			children := current.Children
			left, right := 0, len(children)
			for left < right {
				mid := (left + right) / 2
				if strings.ToLower(children[mid].Name) < strings.ToLower(part) {
					left = mid + 1
				} else {
					right = mid
				}
			}
			if left < len(children) && strings.EqualFold(children[left].Name, part) {
				found = children[left]
			}
			if found == nil {
				fullName := strings.Join(parts[:i+1], "/")
				found = &LabelTreeNode{
					Name:     part,
					FullName: fullName,
				}
				// Insert at sorted position
				current.Children = append(current.Children, nil)
				copy(current.Children[left+1:], current.Children[left:])
				current.Children[left] = found
			}
			current = found
			if i == len(parts)-1 {
				current.MessagesTotal = item.MessagesTotal
				current.MessagesUnread = item.MessagesUnread
				current.IsLabel = item.IsLabel
			}
		}
	}

	// Compute counts in a single pass (no sorting needed)
	for _, child := range root.Children {
		computeRecursiveLabelCounts(child)
	}

	return root.Children
}

func computeRecursiveLabelCounts(n *LabelTreeNode) int64 {
	total := int64(0)
	if n.IsLabel {
		total += n.MessagesTotal
	}
	for _, child := range n.Children {
		total += computeRecursiveLabelCounts(child)
	}
	n.MessagesTotal = total
	return total
}

func FlattenTree(nodes []*LabelTreeNode) []TreeEntry {
	var lines []TreeEntry
	var walk func(n *LabelTreeNode, depth int, isLastSlice []bool)
	walk = func(n *LabelTreeNode, depth int, isLastSlice []bool) {
		var prefix string
		if depth > 0 {
			var sb strings.Builder
			for j := 0; j < depth-1; j++ {
				if isLastSlice[j] {
					sb.WriteString("    ")
				} else {
					sb.WriteString("│   ")
				}
			}
			if isLastSlice[depth-1] {
				sb.WriteString("└── ")
			} else {
				sb.WriteString("├── ")
			}
			prefix = sb.String()
		}
		lines = append(lines, TreeEntry{
			Node:   n,
			Depth:  depth,
			Text:   n.Name,
			Prefix: prefix,
		})
		if n.Expanded {
			for i, c := range n.Children {
				walk(c, depth+1, append(isLastSlice, i == len(n.Children)-1))
			}
		}
	}
	for _, n := range nodes {
		walk(n, 0, []bool{})
	}
	return lines
}

func CountAllDescendants(n *LabelTreeNode) int {
	if n == nil {
		return 0
	}
	count := len(n.Children)
	for _, child := range n.Children {
		count += CountAllDescendants(child)
	}
	return count
}
