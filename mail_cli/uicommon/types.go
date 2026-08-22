package uicommon

import (
	"mail_cli/email"
	"mail_cli/label"
)

type FolderEmail = email.Email
type ThreadNode = email.ThreadNode
type LabelTreeNode = label.LabelTreeNode
type TreeEntry = label.TreeEntry

type Thresholds struct {
	Reject    float64 `json:"reject"`
	AddHeader float64 `json:"add header"`
	Greylist  float64 `json:"greylist"`
}

type Symbol struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

type SpamResponse struct {
	Action     string            `json:"action"`
	Score      float64           `json:"score"`
	Required   float64           `json:"required_score"`
	Thresholds Thresholds        `json:"thresholds"`
	Subject    string            `json:"subject"`
	Symbols    map[string]Symbol `json:"symbols"`
}
