package email

import (
	"sort"
	"time"
)

func BuildThreadTree(emails []Email) []*ThreadNode {
	nodeMap := make(map[string]*ThreadNode)
	var nodes []*ThreadNode

	allocated := make([]*Email, len(emails))
	for i := range emails {
		allocated[i] = &emails[i]
	}

	for _, em := range allocated {
		node := &ThreadNode{
			Email: em,
		}
		nodes = append(nodes, node)
		if em.MessageID != "" {
			nodeMap[em.MessageID] = node
		}
	}

	for _, node := range nodes {
		var parentNode *ThreadNode

		if node.Email.InReplyTo != "" {
			parentIDs := ExtractMessageIDs(node.Email.InReplyTo)
			for _, pid := range parentIDs {
				if p, ok := nodeMap[pid]; ok {
					parentNode = p
					break
				}
			}
		}

		if parentNode == nil && node.Email.References != "" {
			parentIDs := ExtractMessageIDs(node.Email.References)
			for i := len(parentIDs) - 1; i >= 0; i-- {
				pid := parentIDs[i]
				if p, ok := nodeMap[pid]; ok {
					parentNode = p
					break
				}
			}
		}

		if parentNode != nil && parentNode != node {
			node.Parent = parentNode
			parentNode.Children = append(parentNode.Children, node)
		}
	}

	subjectGroups := make(map[string][]*ThreadNode)
	var finalRoots []*ThreadNode

	for _, node := range nodes {
		if node.Parent == nil {
			normSub := CleanSubject(node.Email.Subject)
			if normSub != "" && normSub != "no subject" && normSub != "(no subject)" {
				subjectGroups[normSub] = append(subjectGroups[normSub], node)
			} else {
				finalRoots = append(finalRoots, node)
			}
		}
	}

	for _, group := range subjectGroups {
		if len(group) == 1 {
			finalRoots = append(finalRoots, group[0])
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].Email.EmailDate.Before(group[j].Email.EmailDate)
		})
		root := group[0]
		for i := 1; i < len(group); i++ {
			child := group[i]
			child.Parent = root
			root.Children = append(root.Children, child)
		}
		finalRoots = append(finalRoots, root)
	}

	var sortChildren func(*ThreadNode)
	sortChildren = func(n *ThreadNode) {
		sort.Slice(n.Children, func(i, j int) bool {
			return n.Children[i].Email.EmailDate.Before(n.Children[j].Email.EmailDate)
		})
		for _, child := range n.Children {
			sortChildren(child)
		}
	}
	for _, root := range finalRoots {
		sortChildren(root)
	}

	return finalRoots
}

func GetThreadNewestDate(n *ThreadNode) time.Time {
	newest := n.Email.EmailDate
	var traverse func(node *ThreadNode)
	traverse = func(node *ThreadNode) {
		if node.Email.EmailDate.After(newest) {
			newest = node.Email.EmailDate
		}
		for _, child := range node.Children {
			traverse(child)
		}
	}
	traverse(n)
	return newest
}

func CountThreadReplies(n *ThreadNode) int {
	count := 0
	var traverse func(node *ThreadNode)
	traverse = func(node *ThreadNode) {
		count += len(node.Children)
		for _, child := range node.Children {
			traverse(child)
		}
	}
	traverse(n)
	return count
}

func ExtractMessageIDs(header string) []string {
	var ids []string
	start := -1
	for i, r := range header {
		if r == '<' {
			start = i
		} else if r == '>' && start != -1 {
			ids = append(ids, header[start:i+1])
			start = -1
		}
	}
	return ids
}
