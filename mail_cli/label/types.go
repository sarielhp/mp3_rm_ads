package label

type LabelItem struct {
	Name           string
	FullName       string
	MessagesTotal  int64
	MessagesUnread int64
	IsLabel        bool
}

type LabelTreeNode struct {
	Name           string
	FullName       string
	MessagesTotal  int64
	MessagesUnread int64
	IsLabel        bool
	Children       []*LabelTreeNode
	Expanded       bool
}

type TreeEntry struct {
	Node   *LabelTreeNode
	Depth  int
	Text   string
	Prefix string
}

// MessageFolderRef pairs a message ID with its associated folder name.
type MessageFolderRef struct {
	MessageID string
	Folder    string
}
