package email

import "time"

type Email struct {
	ID                  string
	Subject             string
	FromEmail           string
	FromRaw             string
	EmailDate           time.Time
	FormattedDate       string
	IsSpam              bool
	IsPolitical         bool
	IsBlacklisted       bool
	IsRead              bool
	IsReplied           bool
	HasICS              bool
	HasAttachment       bool
	MessageID           string
	InReplyTo           string
	References          string
	ThreadDepth         int
	ThreadHasReplies    bool
	ThreadCollapsed     bool
	ThreadRepliesCount  int
	ThreadSenderSummary string
	ThreadPrefix        string
	ThreadIndex         int
}

type ThreadNode struct {
	Email    *Email
	Children []*ThreadNode
	Parent   *ThreadNode
}
