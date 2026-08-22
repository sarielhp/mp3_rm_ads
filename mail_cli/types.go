package main

import (
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
	"mail_cli/uicommon"
)

type Config = cfg_g.Config
type FileConfig = cfg_g.FileConfig
type AccountConfig = cfg_acc.AccountConfig
type Rule = cfg_acc.Rule
type LabelItem = cfg_acc.LabelItem
type SpamThresholds = uicommon.Thresholds
type SpamSymbol = uicommon.Symbol
type SpamResponse = uicommon.SpamResponse
