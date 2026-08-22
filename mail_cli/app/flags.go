package app

import "errors"

var (
	FlagMoveSpamStr     string
	FlagMoveInboxStr    string
	FlagScanPattern     string
	FlagAccountName     string
	FlagRuleExportFile  string
	FlagRuleImportFile  string
	FlagRuleListAll     bool
	FlagSieveExport     string
	FlagLabelsListAll   bool
	FlagForceLearnHam   bool
	FlagForceRuleExport bool
	FlagShowWeb         bool

	FlagVerbose   bool
	FlagVersion   bool
	FlagLogAppend bool
	FlagFixConfig bool
	FlagReadOnly  bool

	FlagExplicitScanInbox bool

	CmdExecutionStarted bool
	TuiActive           bool
)

var ErrUsage = errors.New("usage error")
