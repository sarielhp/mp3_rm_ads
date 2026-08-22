package tui

import (
	"sort"
	"strings"

	"mail_cli/email"
	"mail_cli/label"
	"mail_cli/uicommon"
)

func (m *tuiModel) buildEntries() {
	var tree []*uicommon.LabelTreeNode
	if m.inFolderSearch && m.folderSearch != "" {
		var filtered []label.LabelItem
		lowerSearch := strings.ToLower(m.folderSearch)
		sourceFolders := m.folders
		if m.showGlobalTree {
			sourceFolders = m.globalFolders
		}
		for _, f := range sourceFolders {
			if strings.Contains(strings.ToLower(f.FullName), lowerSearch) {
				filtered = append(filtered, f)
			}
		}
		tree = uicommon.BuildLabelTree(filtered)
		var expandAll func(nodes []*uicommon.LabelTreeNode)
		expandAll = func(nodes []*uicommon.LabelTreeNode) {
			for _, n := range nodes {
				n.Expanded = true
				expandAll(n.Children)
			}
		}
		expandAll(tree)
	} else {
		if m.showGlobalTree {
			tree = m.globalTree
		} else {
			tree = m.tree
		}
	}
	m.entries = uicommon.FlattenTree(tree)
	if m.inFolderSearch && m.folderSearch != "" {
		found := false
		for i, entry := range m.entries {
			if entry.Node != nil && len(entry.Node.Children) == 0 {
				m.cursor = i
				found = true
				break
			}
		}
		if !found {
			m.cursor = min(m.cursor, max(0, len(m.entries)-1))
		}
	} else {
		m.cursor = min(m.cursor, max(0, len(m.entries)-1))
	}
}

func (m *tuiModel) rebuildVisibleEmails() {
	if len(m.rawEmails) == 0 {
		m.emails = nil
		return
	}
	roots := email.BuildThreadTree(m.rawEmails)
	isBadNode := func(n *email.ThreadNode) bool {
		return n.Email.IsSpam || n.Email.IsPolitical || n.Email.IsBlacklisted
	}
	sort.Slice(roots, func(i, j int) bool {
		badI := isBadNode(roots[i])
		badJ := isBadNode(roots[j])
		if badI != badJ {
			return !badI
		}
		dateI := email.GetThreadNewestDate(roots[i])
		dateJ := email.GetThreadNewestDate(roots[j])
		return dateI.Before(dateJ)
	})
	var expandedOrder []string
	var getExpandedOrder func(n *uicommon.ThreadNode)
	getExpandedOrder = func(n *uicommon.ThreadNode) {
		expandedOrder = append(expandedOrder, n.Email.ID)
		for _, child := range n.Children {
			getExpandedOrder(child)
		}
	}
	for _, root := range roots {
		getExpandedOrder(root)
	}
	idToNumber := make(map[string]int)
	for i, id := range expandedOrder {
		idToNumber[id] = i + 1
	}
	var visible []uicommon.FolderEmail
	var flatten func(n *uicommon.ThreadNode, depth int, isAncestorCollapsed bool, isLastSlice []bool)
	flatten = func(n *uicommon.ThreadNode, depth int, isAncestorCollapsed bool, isLastSlice []bool) {
		if isAncestorCollapsed {
			for i, child := range n.Children {
				flatten(child, depth+1, true, append(isLastSlice, i == len(n.Children)-1))
			}
			return
		}
		em := *n.Email
		em.ThreadDepth = depth
		em.ThreadHasReplies = len(n.Children) > 0
		em.ThreadRepliesCount = email.CountThreadReplies(n)
		em.ThreadIndex = idToNumber[em.ID]
		key := em.MessageID
		if key == "" {
			key = em.ID
		}
		isCollapsed := !m.expandedThreads[key]
		em.ThreadCollapsed = isCollapsed
		if depth == 0 {
			if em.ThreadHasReplies {
				if isCollapsed {
					em.ThreadPrefix = "━━━ "
				} else {
					em.ThreadPrefix = "┌── "
				}
			}
		} else {
			var sb strings.Builder
			for j := 0; j < depth-1; j++ {
				if isLastSlice[j] {
					sb.WriteString("  ")
				} else {
					sb.WriteString("│ ")
				}
			}
			if isLastSlice[depth-1] {
				sb.WriteString("└── ")
			} else {
				sb.WriteString("├── ")
			}
			em.ThreadPrefix = sb.String()
		}
		if em.ThreadHasReplies {
			var senders []string
			seen := make(map[string]bool)
			var collectSenders func(node *uicommon.ThreadNode)
			collectSenders = func(node *uicommon.ThreadNode) {
				name := uicommon.FormatSender(node.Email.FromRaw)
				if !seen[name] {
					seen[name] = true
					senders = append(senders, name)
				}
				for _, child := range node.Children {
					collectSenders(child)
				}
			}
			collectSenders(n)
			em.ThreadSenderSummary = strings.Join(senders, ", ")
		}
		visible = append(visible, em)
		for i, child := range n.Children {
			flatten(child, depth+1, isCollapsed, append(isLastSlice, i == len(n.Children)-1))
		}
	}
	for _, root := range roots {
		flatten(root, 0, false, []bool{})
	}
	if m.searchQuery != "" {
		if m.fuzzySearch {
			visible = fuzzyFilterEmails(m.searchQuery, visible)
		} else {
			var filtered []uicommon.FolderEmail
			lowerQuery := strings.ToLower(m.searchQuery)
			for _, em := range visible {
				if strings.Contains(strings.ToLower(em.Subject), lowerQuery) ||
					strings.Contains(strings.ToLower(em.FromRaw), lowerQuery) ||
					strings.Contains(strings.ToLower(em.FromEmail), lowerQuery) {
					filtered = append(filtered, em)
				}
			}
			visible = filtered
		}
	}
	m.emails = visible
}

func (m *tuiModel) expandRecursively(messageID string, emailID string) {
	roots := email.BuildThreadTree(m.rawEmails)
	var findNode func(nodes []*uicommon.ThreadNode) *uicommon.ThreadNode
	findNode = func(nodes []*uicommon.ThreadNode) *uicommon.ThreadNode {
		for _, n := range nodes {
			key := n.Email.MessageID
			if key == "" {
				key = n.Email.ID
			}
			if (messageID != "" && key == messageID) || n.Email.ID == emailID {
				return n
			}
			if found := findNode(n.Children); found != nil {
				return found
			}
		}
		return nil
	}
	target := findNode(roots)
	if target == nil {
		return
	}
	var markExpanded func(n *uicommon.ThreadNode)
	markExpanded = func(n *uicommon.ThreadNode) {
		key := n.Email.MessageID
		if key == "" {
			key = n.Email.ID
		}
		m.expandedThreads[key] = true
		for _, child := range n.Children {
			markExpanded(child)
		}
	}
	markExpanded(target)
}

func (m *tuiModel) cycleTreeSubtreeExpansion(root *uicommon.LabelTreeNode) {
	if root == nil || len(root.Children) == 0 {
		return
	}

	// Find the shallowest depth of any node with children in root's subtree that is not yet expanded.
	minUnexpandedDepth := -1
	var findShallowest func(n *uicommon.LabelTreeNode, depth int)
	findShallowest = func(n *uicommon.LabelTreeNode, depth int) {
		if len(n.Children) > 0 {
			if !n.Expanded {
				if minUnexpandedDepth == -1 || depth < minUnexpandedDepth {
					minUnexpandedDepth = depth
				}
			}
			for _, child := range n.Children {
				findShallowest(child, depth+1)
			}
		}
	}
	findShallowest(root, 0)

	// If all nodes with children in the subtree are already expanded, fold/collapse the whole subtree.
	if minUnexpandedDepth == -1 {
		var collapseSubtree func(n *uicommon.LabelTreeNode)
		collapseSubtree = func(n *uicommon.LabelTreeNode) {
			n.Expanded = false
			for _, child := range n.Children {
				collapseSubtree(child)
			}
		}
		collapseSubtree(root)
		return
	}

	// Otherwise, expand all nodes with children in the subtree at relative depth <= minUnexpandedDepth.
	var expandToDepth func(n *uicommon.LabelTreeNode, depth int)
	expandToDepth = func(n *uicommon.LabelTreeNode, depth int) {
		if len(n.Children) > 0 {
			if depth <= minUnexpandedDepth {
				n.Expanded = true
			}
			for _, child := range n.Children {
				expandToDepth(child, depth+1)
			}
		}
	}
	expandToDepth(root, 0)
}

func (m *tuiModel) cycleEmailThreadSubtreeExpansion(emailID string, messageID string) {
	roots := email.BuildThreadTree(m.rawEmails)
	var findNode func(nodes []*uicommon.ThreadNode) *uicommon.ThreadNode
	findNode = func(nodes []*uicommon.ThreadNode) *uicommon.ThreadNode {
		for _, n := range nodes {
			key := n.Email.MessageID
			if key == "" {
				key = n.Email.ID
			}
			if (messageID != "" && key == messageID) || n.Email.ID == emailID {
				return n
			}
			if found := findNode(n.Children); found != nil {
				return found
			}
		}
		return nil
	}
	target := findNode(roots)
	if target == nil || len(target.Children) == 0 {
		return
	}

	nodeKey := func(n *uicommon.ThreadNode) string {
		if n.Email.MessageID != "" {
			return n.Email.MessageID
		}
		return n.Email.ID
	}

	// Find the shallowest depth of any node with children in target's subtree that is not yet expanded.
	minUnexpandedDepth := -1
	var findShallowest func(n *uicommon.ThreadNode, depth int)
	findShallowest = func(n *uicommon.ThreadNode, depth int) {
		if len(n.Children) > 0 {
			if !m.expandedThreads[nodeKey(n)] {
				if minUnexpandedDepth == -1 || depth < minUnexpandedDepth {
					minUnexpandedDepth = depth
				}
			}
			for _, child := range n.Children {
				findShallowest(child, depth+1)
			}
		}
	}
	findShallowest(target, 0)

	// If all nodes with children in the subtree are already expanded, fold/collapse the whole subtree.
	if minUnexpandedDepth == -1 {
		var collapseSubtree func(n *uicommon.ThreadNode)
		collapseSubtree = func(n *uicommon.ThreadNode) {
			delete(m.expandedThreads, nodeKey(n))
			for _, child := range n.Children {
				collapseSubtree(child)
			}
		}
		collapseSubtree(target)
	} else {
		// Otherwise, expand all nodes with children in the subtree at relative depth <= minUnexpandedDepth.
		var expandToDepth func(n *uicommon.ThreadNode, depth int)
		expandToDepth = func(n *uicommon.ThreadNode, depth int) {
			if len(n.Children) > 0 {
				if depth <= minUnexpandedDepth {
					m.expandedThreads[nodeKey(n)] = true
				}
				for _, child := range n.Children {
					expandToDepth(child, depth+1)
				}
			}
		}
		expandToDepth(target, 0)
	}

	currentID := emailID
	m.rebuildVisibleEmails()
	for i, e := range m.emails {
		if e.ID == currentID {
			m.eIdx = i
			break
		}
	}
}
