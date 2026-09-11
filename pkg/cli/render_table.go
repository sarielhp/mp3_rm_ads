package cli

import (
	"abs/pkg/format"
	"abs/pkg/util"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type TableAlign int

const (
	AlignLeft TableAlign = iota
	AlignCenter
	AlignRight
)

type TableColumn struct {
	Header string
	Width  int
	Align  TableAlign
}

func stringDisplayWidth(s string) int {
	clean := stripAnsi(s)
	w := 0
	for _, r := range clean {
		w += runeDisplayWidth(r)
	}
	return w
}

func stripAnsi(s string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	var sb strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			inEsc = true
			i++
			continue
		}
		if inEsc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEsc = false
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func runeDisplayWidth(r rune) int {
	if r == 0xFE0F || r == 0xFE0E || (r >= 0x200B && r <= 0x200D) || (r >= 0x0300 && r <= 0x036F) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	if (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2300 && r <= 0x23FF) {
		return true
	}
	if r == 0x2728 || r == 0x2702 || r == 0x2B07 || r == 0x26A1 {
		return true
	}
	return r >= 0x4E00 && r <= 0x9FFF
}

func padCell(val string, width int, align TableAlign) string {
	w := stringDisplayWidth(val)
	if w >= width {
		return val
	}
	diff := width - w
	switch align {
	case AlignCenter:
		left := diff / 2
		right := diff - left
		return strings.Repeat(" ", left) + val + strings.Repeat(" ", right)
	case AlignRight:
		return strings.Repeat(" ", diff) + val
	default:
		return val + strings.Repeat(" ", diff)
	}
}

func renderTableTop(cols []TableColumn) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, strings.Repeat("─", c.Width+2))
	}
	return "┌" + strings.Join(parts, "┬") + "┐"
}

func renderTableHeader(cols []TableColumn) string {
	var formatted []string
	for _, c := range cols {
		formatted = append(formatted, padCell(c.Header, c.Width, AlignCenter))
	}
	return "│ " + strings.Join(formatted, " │ ") + " │"
}

func renderTableDivider(cols []TableColumn) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, strings.Repeat("─", c.Width+2))
	}
	return "├" + strings.Join(parts, "┼") + "┤"
}

func renderTableRow(cells []string, cols []TableColumn) string {
	var formatted []string
	for i, c := range cols {
		val := ""
		if i < len(cells) {
			val = cells[i]
		}
		formatted = append(formatted, padCell(val, c.Width, c.Align))
	}
	return "│ " + strings.Join(formatted, " │ ") + " │"
}

func renderTableBottom(cols []TableColumn) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, strings.Repeat("─", c.Width+2))
	}
	return "└" + strings.Join(parts, "┴") + "┘"
}

func compactDownloadPolicy(policy string, k int) string {
	switch strings.ToLower(policy) {
	case DownloadPolicyLatest:
		return "New"
	case DownloadPolicyAll:
		return "All"
	case DownloadPolicyNone:
		return "Off"
	case DownloadPolicyLatestK:
		if k <= 0 {
			k = 3
		}
		return fmt.Sprintf("Top %d", k)
	default:
		if policy == "" {
			return "New"
		}
		return policy
	}
}

func compactAdRemoval(adRemoval string) string {
	switch strings.ToLower(adRemoval) {
	case AdRemovalAll:
		return "All"
	case AdRemovalNone:
		return "Off"
	case AdRemovalLatest:
		return "New"
	default:
		if adRemoval == "" {
			return "All"
		}
		return adRemoval
	}
}

func podcastTableColumns(titleWidth int) []TableColumn {
	return []TableColumn{
		{Header: "ID", Width: 5, Align: AlignCenter},
		{Header: "Title", Width: titleWidth, Align: AlignLeft},
		{Header: "🎙️", Width: 4, Align: AlignRight},
		{Header: "✨", Width: 4, Align: AlignRight},
		{Header: "⬇️", Width: 5, Align: AlignCenter},
		{Header: "✂️", Width: 5, Align: AlignCenter},
		{Header: "⏳", Width: 4, Align: AlignCenter},
		{Header: "📅 Last", Width: 16, Align: AlignCenter},
	}
}

func latestEpisodeTableColumns(podWidth, titleWidth int) []TableColumn {
	return []TableColumn{
		{Header: "📅 Date", Width: 16, Align: AlignCenter},
		{Header: "🎙️ Pod", Width: 6, Align: AlignCenter},
		{Header: "🔖 Ep", Width: 6, Align: AlignCenter},
		{Header: "📻 Podcast", Width: podWidth, Align: AlignLeft},
		{Header: "✂️ AdR", Width: 9, Align: AlignCenter},
		{Header: "⏱️ Dur", Width: 6, Align: AlignRight},
		{Header: "Title", Width: titleWidth, Align: AlignLeft},
	}
}

func buildPodcastRowCells(item lsPodcastItem, titleWidth int) []string {
	pName := util.TruncateDisplayName(item.Title, titleWidth)
	dlStr := compactDownloadPolicy(item.DownloadPolicy, 3)
	adStr := compactAdRemoval(item.AdRemoval)
	lastDateStr := formatRelativeDateStr(item.LastEpisode)
	return []string{
		util.BoldCyan(item.ShortID),
		pName,
		strconv.Itoa(item.EpisodeCount),
		strconv.Itoa(item.CleanCount),
		dlStr,
		adStr,
		item.Retention,
		lastDateStr,
	}
}

func buildLatestEpisodeRowCells(item lsEpisodeItem, podWidth, titleWidth int) []string {
	dStr := formatRelativeDateTime(item.modTime)
	pName := util.TruncateDisplayName(item.podcastTitle, podWidth)
	shortStatus := formatShortStatus(item.statusStr)
	coloredStatus := shortStatus
	if item.statusColor == "green" {
		coloredStatus = util.BoldGreen(shortStatus)
	} else if item.statusColor == "yellow" {
		coloredStatus = util.BoldYellow(shortStatus)
	} else if item.statusColor == "cyan" {
		coloredStatus = util.Bold(shortStatus)
	}
	durStr := "-"
	if item.origDuration > 0 {
		durStr = format.FormatClock(item.origDuration)
	}
	epName := util.TruncateDisplayName(item.episodeName, titleWidth)
	return []string{
		dStr,
		item.podcastShortID,
		util.BoldCyan(item.episodeShortID),
		pName,
		coloredStatus,
		durStr,
		epName,
	}
}

func formatRelativeDateAt(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}

	loc := now.Location()
	tLoc := t.In(loc)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	targetDay := time.Date(tLoc.Year(), tLoc.Month(), tLoc.Day(), 0, 0, 0, 0, loc)

	if targetDay.After(today) {
		return tLoc.Format("2006-01-02")
	}

	diffHours := today.Sub(targetDay).Hours()
	days := int(diffHours+12) / 24

	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days >= 2 && days <= 7:
		return fmt.Sprintf("last week (-%d d)", days)
	default:
		return tLoc.Format("2006-01-02")
	}
}

func formatRelativeDateStrAt(dateStr string, now time.Time) string {
	if dateStr == "" || dateStr == "-" {
		return "-"
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04", time.RFC3339} {
		if pt, err := time.ParseInLocation(layout, dateStr, now.Location()); err == nil {
			return formatRelativeDateAt(pt, now)
		}
	}
	return dateStr
}

func formatRelativeDateStr(dateStr string) string {
	return formatRelativeDateStrAt(dateStr, time.Now())
}

func formatRelativeDateTimeAt(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}

	loc := now.Location()
	tLoc := t.In(loc)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	targetDay := time.Date(tLoc.Year(), tLoc.Month(), tLoc.Day(), 0, 0, 0, 0, loc)

	if targetDay.After(today) {
		return tLoc.Format("2006-01-02 15:04")
	}

	diffHours := today.Sub(targetDay).Hours()
	days := int(diffHours+12) / 24

	switch {
	case days == 0:
		return fmt.Sprintf("today %s", tLoc.Format("15:04"))
	case days == 1:
		return fmt.Sprintf("yesterday %s", tLoc.Format("15:04"))
	case days >= 2 && days <= 7:
		return fmt.Sprintf("last week (-%d d)", days)
	default:
		return tLoc.Format("2006-01-02 15:04")
	}
}

func formatRelativeDateTime(t time.Time) string {
	return formatRelativeDateTimeAt(t, time.Now())
}

func queueTableColumns(titleWidth int) []TableColumn {
	return []TableColumn{
		{Header: "Podcast ID", Width: 10, Align: AlignCenter},
		{Header: "Episode ID", Width: 10, Align: AlignCenter},
		{Header: "Pri", Width: 3, Align: AlignRight},
		{Header: "Length", Width: 8, Align: AlignRight},
		{Header: "P-date", Width: 19, Align: AlignLeft},
		{Header: "Title", Width: titleWidth, Align: AlignLeft},
	}
}
