package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/cfg_g"
	"mail_cli/email"
	"mail_cli/mailclient"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// ICSEvent represents a parsed calendar event from an ICS attachment.
type ICSEvent struct {
	UID         string
	Summary     string
	Description string
	Location    string
	StartStr    string
	EndStr      string
	StartParsed time.Time
	EndParsed   time.Time
	IsAllDay    bool
}

// unfoldICS unfolds folded lines in an ICS file.
func unfoldICS(input string) string {
	r := strings.NewReplacer("\r\n ", "", "\r\n\t", "", "\n ", "", "\n\t", "")
	return r.Replace(input)
}

// parseICSDateTime parses ICS format datetime strings.
func parseICSDateTime(val string, tzid string) (time.Time, bool, error) {
	val = strings.TrimSpace(val)
	if len(val) == 8 {
		t, err := time.Parse("20060102", val)
		if err == nil {
			return t, true, nil
		}
	}
	if strings.HasSuffix(val, "Z") {
		t, err := time.Parse("20060102T150405Z", val)
		if err == nil {
			return t, false, nil
		}
	}
	if tzid != "" {
		loc, err := time.LoadLocation(tzid)
		if err == nil {
			t, err := time.ParseInLocation("20060102T150405", val, loc)
			if err == nil {
				return t, false, nil
			}
		}
	}
	t, err := time.Parse("20060102T150405", val)
	if err == nil {
		return t, false, nil
	}
	return time.Time{}, false, fmt.Errorf("unsupported datetime format: %s", val)
}

// parseICSLine splits a single ICS line into key, params map, and value.
func parseICSLine(line string) (key string, params map[string]string, value string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return "", nil, ""
	}
	keyPart := parts[0]
	value = parts[1]

	key = keyPart
	params = make(map[string]string)
	if strings.Contains(keyPart, ";") {
		subparts := strings.Split(keyPart, ";")
		key = subparts[0]
		for _, p := range subparts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				params[strings.ToUpper(kv[0])] = kv[1]
			}
		}
	}
	return
}

// extractICSEventFields extracts raw VEVENT fields from parsed ICS lines.
func extractICSEventFields(lines []string) (*ICSEvent, string, string, error) {
	inEvent := false
	event := &ICSEvent{}
	var startTzid, endTzid string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "BEGIN:VEVENT") {
			inEvent = true
			continue
		}
		if strings.EqualFold(line, "END:VEVENT") {
			break
		}
		if !inEvent {
			continue
		}

		key, params, val := parseICSLine(line)
		if key == "" {
			continue
		}

		valClean := strings.NewReplacer(`\,`, `,`, `\;`, `;`, `\n`, "\n", `\N`, "\n", `\\`, `\`).Replace(val)

		switch strings.ToUpper(key) {
		case "UID":
			event.UID = strings.TrimSpace(valClean)
		case "SUMMARY":
			event.Summary = strings.TrimSpace(valClean)
		case "DESCRIPTION":
			event.Description = strings.TrimSpace(valClean)
		case "LOCATION":
			event.Location = strings.TrimSpace(valClean)
		case "DTSTART":
			event.StartStr = strings.TrimSpace(val)
			startTzid = params["TZID"]
		case "DTEND":
			event.EndStr = strings.TrimSpace(val)
			endTzid = params["TZID"]
		}
	}

	if event.StartStr == "" {
		return nil, "", "", fmt.Errorf("ICS event is missing DTSTART")
	}
	return event, startTzid, endTzid, nil
}

// parseFirstICSEvent extracts and parses the first VEVENT block from an ICS string.
func parseFirstICSEvent(icsContent string) (*ICSEvent, error) {
	unfolded := unfoldICS(icsContent)
	lines := strings.Split(unfolded, "\n")

	event, startTzid, endTzid, err := extractICSEventFields(lines)
	if err != nil {
		return nil, err
	}

	tStart, isAllDay, err := parseICSDateTime(event.StartStr, startTzid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DTSTART %q: %w", event.StartStr, err)
	}
	event.StartParsed = tStart
	event.IsAllDay = isAllDay

	if event.EndStr != "" {
		tEnd, _, err := parseICSDateTime(event.EndStr, endTzid)
		if err == nil {
			event.EndParsed = tEnd
		} else {
			event.EndParsed = event.StartParsed.Add(1 * time.Hour)
		}
	} else {
		if event.IsAllDay {
			event.EndParsed = event.StartParsed.AddDate(0, 0, 1)
		} else {
			event.EndParsed = event.StartParsed.Add(1 * time.Hour)
		}
	}

	return event, nil
}

// extractICSAttachmentFromMsg parses MIME structure recursively to find an ICS attachment.
func extractICSAttachmentFromMsg(header mail.Header, body io.Reader) ([]byte, error) {
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		ct := strings.ToLower(header.Get("Content-Type"))
		if strings.Contains(ct, "text/calendar") || strings.Contains(ct, "application/ics") || strings.Contains(ct, ".ics") {
			encoding := header.Get("Content-Transfer-Encoding")
			return decodePartBodyReader(body, encoding)
		}
		return nil, nil
	}

	if name, ok := params["name"]; ok && strings.HasSuffix(strings.ToLower(name), ".ics") {
		encoding := header.Get("Content-Transfer-Encoding")
		return decodePartBodyReader(body, encoding)
	}

	cdHeader := header.Get("Content-Disposition")
	if cdHeader != "" {
		cdType, cdParams, err := mime.ParseMediaType(cdHeader)
		if err == nil {
			if cdType == "attachment" {
				if filename, ok := cdParams["filename"]; ok && strings.HasSuffix(strings.ToLower(filename), ".ics") {
					encoding := header.Get("Content-Transfer-Encoding")
					return decodePartBodyReader(body, encoding)
				}
			}
		}
	}

	if mediaType == "text/calendar" || mediaType == "application/ics" || mediaType == "text/x-vcalendar" {
		encoding := header.Get("Content-Transfer-Encoding")
		return decodePartBodyReader(body, encoding)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, nil
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			partHeader := mail.Header(part.Header)
			data, err := extractICSAttachmentFromMsg(partHeader, part)
			if err == nil && data != nil {
				return data, nil
			}
		}
	}

	return nil, nil
}

// decodePartBodyReader handles body decoding based on transfer encoding header.
func decodePartBodyReader(r io.Reader, encoding string) ([]byte, error) {
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	if encoding == "base64" {
		dec := base64.NewDecoder(base64.StdEncoding, r)
		return io.ReadAll(dec)
	} else if encoding == "quoted-printable" {
		dec := quotedprintable.NewReader(r)
		return io.ReadAll(dec)
	}
	return io.ReadAll(r)
}

// findCalendarManagerAccount finds the configured account designated as calendar manager.
func findCalendarManagerAccount(config *Config) (*AccountConfig, error) {
	for i, acc := range config.Accounts {
		if acc.CalendarManager {
			return &config.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("no calendar manager account designated. Use 'mail_cli account calendar <name>' to designate one")
}

// getCalendarService returns the authenticated Google Calendar service.
func getCalendarService(config *Config) (*calendar.Service, error) {
	calAcc, err := findCalendarManagerAccount(config)
	if err != nil {
		return nil, err
	}

	origSelected := config.SelectedAccount
	config.SelectedAccount = calAcc.Name
	defer func() {
		config.SelectedAccount = origSelected
	}()

	configDir := config.ConfigDir
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", app.AppName)
	}
	tokenPath := GetTokenPath(config)

	oauthConfig, err := getOauthConfig(configDir, oauthScopes)
	if err != nil {
		return nil, err
	}

	client, err := getOAuthClient(oauthConfig, tokenPath, config)
	if err != nil {
		return nil, err
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %w", err)
	}

	return srv, nil
}

// checkAndAddEvent verifies if event exists, adds it if not, and returns status.
func checkAndAddEvent(srv *calendar.Service, event *ICSEvent) (*calendar.Event, bool, error) {
	if event.UID != "" {
		list, err := srv.Events.List("primary").ICalUID(event.UID).Do()
		if err == nil && len(list.Items) > 0 {
			return list.Items[0], true, nil
		}
	}

	timeMin := event.StartParsed.Add(-1 * time.Hour).Format(time.RFC3339)
	timeMax := event.StartParsed.Add(1 * time.Hour).Format(time.RFC3339)
	list, err := srv.Events.List("primary").
		TimeMin(timeMin).
		TimeMax(timeMax).
		SingleEvents(true).
		Do()
	if err == nil {
		for _, item := range list.Items {
			if strings.EqualFold(item.Summary, event.Summary) {
				return item, true, nil
			}
		}
	}

	newEvent := &calendar.Event{
		Summary:     event.Summary,
		Description: event.Description,
		Location:    event.Location,
		ICalUID:     event.UID,
	}

	if event.IsAllDay {
		newEvent.Start = &calendar.EventDateTime{
			Date: event.StartParsed.Format("2006-01-02"),
		}
		newEvent.End = &calendar.EventDateTime{
			Date: event.EndParsed.Format("2006-01-02"),
		}
	} else {
		newEvent.Start = &calendar.EventDateTime{
			DateTime: event.StartParsed.Format(time.RFC3339),
		}
		newEvent.End = &calendar.EventDateTime{
			DateTime: event.EndParsed.Format(time.RFC3339),
		}
	}

	created, err := srv.Events.Insert("primary", newEvent).Do()
	if err != nil {
		return nil, false, err
	}

	return created, false, nil
}

// listEventsForDay lists all events for the date of the given time.
func listEventsForDay(srv *calendar.Service, t time.Time) ([]*calendar.Event, error) {
	loc := t.Location()
	year, month, day := t.Date()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)

	list, err := srv.Events.List("primary").
		TimeMin(dayStart.Format(time.RFC3339)).
		TimeMax(dayEnd.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// printDayEvents formats and outputs calendar events to stdout.
func printDayEvents(events []*calendar.Event, dayStart time.Time, targetEvent *calendar.Event) {
	app.ColorBoldCyan.Println("\n======================================================================")
	app.ColorBoldCyan.Printf("                     CALENDAR SCHEDULE FOR %s                  \n", dayStart.Format("2006-01-02"))
	app.ColorBoldCyan.Println("======================================================================")

	if len(events) == 0 {
		fmt.Println("  No events scheduled for this day.")
	} else {
		for _, ev := range events {
			timeStr := ""
			if ev.Start.Date != "" {
				timeStr = app.ColorBoldYellow.Sprint("ALL DAY")
			} else {
				st, err1 := time.Parse(time.RFC3339, ev.Start.DateTime)
				et, err2 := time.Parse(time.RFC3339, ev.End.DateTime)
				if err1 == nil && err2 == nil {
					timeStr = fmt.Sprintf("%s - %s", st.Format("15:04"), et.Format("15:04"))
				} else {
					timeStr = "??:??"
				}
			}

			summary := ev.Summary
			if summary == "" {
				summary = "(No Title)"
			}

			if targetEvent != nil && (ev.Id == targetEvent.Id || (ev.ICalUID != "" && ev.ICalUID == targetEvent.ICalUID)) {
				summary = app.ColorBoldGreen.Sprint(summary) + " " + app.ColorBoldGreen.Sprint("[TARGET]")
			}

			locStr := ""
			if ev.Location != "" {
				locStr = fmt.Sprintf(" (%s)", ev.Location)
			}

			fmt.Printf("  [%s] %s%s\n", timeStr, summary, locStr)
		}
	}
	app.ColorBoldCyan.Println("======================================================================")
	fmt.Println()
}

// printEventSummary prints an ICS event's summary, start time, and location to stdout.
func printEventSummary(event *ICSEvent) {
	app.ColorBoldYellow.Printf("  Summary:     ")
	fmt.Println(event.Summary)
	app.ColorBoldYellow.Printf("  Start:       ")
	if event.IsAllDay {
		fmt.Println(event.StartParsed.Format("2006-01-02") + " (All Day)")
	} else {
		fmt.Println(event.StartParsed.Format("2006-01-02 15:04:05 MST"))
	}
	if event.Location != "" {
		app.ColorBoldYellow.Printf("  Location:    ")
		fmt.Println(event.Location)
	}
}

// findICSDataForMessage searches for an ICS attachment in a specific message across labels.
func findICSDataForMessage(client mailclient.MailClient, localCfg *Config, matchedLabels []string, targetMsgID string) ([]byte, string, error) {
	for _, matchedLabel := range matchedLabels {
		cacheDirName := cfg_g.SanitizeLabelForCache(matchedLabel)

		downloadedIDs, err := client.FetchAndDownloadEmails(matchedLabel, cacheDirName)
		if err != nil {
			return nil, "", fmt.Errorf("error downloading emails for label %s: %w", matchedLabel, err)
		}

		targetUpper := strings.ToUpper(strings.TrimSpace(targetMsgID))
		var matchedID string
		for _, id := range downloadedIDs {
			if strings.ToUpper(id) == targetUpper || email.ComputeShortID(id) == targetUpper {
				matchedID = id
				break
			}
		}

		if matchedID != "" {
			rawBytes, rErr := msg.Read(localCfg.DownloadDir, matchedID)
			if rErr != nil {
				continue
			}
			m, rErr := mail.ReadMessage(bytes.NewReader(rawBytes))
			if rErr != nil {
				continue
			}
			data, err := extractICSAttachmentFromMsg(m.Header, m.Body)
			if err != nil {
				return nil, "", fmt.Errorf("failed to extract ICS attachment from %s: %w", matchedID, err)
			}
			if data != nil {
				return data, matchedLabel, nil
			}
		}
	}
	return nil, "", fmt.Errorf("could not find any .ics attachment in message %s within labels matching %q", targetMsgID, matchedLabels)
}

// PerformCalendarAdd extracts an .ics file from a message and inserts it into the designated calendar manager's calendar.
func PerformCalendarAdd(config *Config, client mailclient.MailClient, labelPrefix string, targetMsgID string) error {
	localCfg := client.Config()

	matchedLabels, err := client.GetMatchingLabels(labelPrefix)
	if err != nil {
		return err
	}
	if len(matchedLabels) == 0 {
		return fmt.Errorf("no labels found matching prefix %q", labelPrefix)
	}

	icsData, foundLabel, err := findICSDataForMessage(client, localCfg, matchedLabels, targetMsgID)
	if err != nil {
		return err
	}

	event, err := parseFirstICSEvent(string(icsData))
	if err != nil {
		return fmt.Errorf("failed to parse ICS event: %w", err)
	}

	fmt.Printf("%s Found calendar event in message %s (folder %s):\n", app.PrefixInfo, targetMsgID, foundLabel)
	printEventSummary(event)

	srv, err := getCalendarService(config)
	if err != nil {
		return fmt.Errorf("failed to load calendar service: %w", err)
	}

	calEvent, existed, err := checkAndAddEvent(srv, event)
	if err != nil {
		return fmt.Errorf("failed to process calendar event check/addition: %w", err)
	}

	if existed {
		fmt.Printf("%s Event is already present in your calendar.\n", app.PrefixWarn)
	} else {
		fmt.Printf("%s Successfully added event to your calendar!\n", app.PrefixSuccess)
	}

	dayEvents, err := listEventsForDay(srv, event.StartParsed)
	if err != nil {
		return fmt.Errorf("failed to fetch schedule list for the event day: %w", err)
	}

	printDayEvents(dayEvents, event.StartParsed, calEvent)
	return nil
}

// processCalendarMessage processes a single cached message for calendar events.
func processCalendarMessage(srv *calendar.Service, config *Config, localCfg *Config, id string) (processed bool, added bool) {
	rawBytes, rErr := msg.Read(localCfg.DownloadDir, id)
	if rErr != nil {
		return false, false
	}
	m, rErr := mail.ReadMessage(bytes.NewReader(rawBytes))
	if rErr != nil {
		return false, false
	}
	icsData, err := extractICSAttachmentFromMsg(m.Header, m.Body)
	if err != nil || len(icsData) == 0 {
		fmt.Printf("%s [Message %s] Failed to extract ICS: %v\n", app.PrefixError, email.ComputeShortID(id), err)
		return false, false
	}

	event, err := parseFirstICSEvent(string(icsData))
	if err != nil {
		fmt.Printf("%s [Message %s] Failed to parse ICS: %v\n", app.PrefixError, email.ComputeShortID(id), err)
		return false, false
	}

	fmt.Printf("%s Found calendar event in message %s:\n", app.PrefixInfo, email.ComputeShortID(id))
	printEventSummary(event)

	calEvent, existed, err := checkAndAddEvent(srv, event)
	if err != nil {
		fmt.Printf("%s [Message %s] Failed to process calendar: %v\n", app.PrefixError, email.ComputeShortID(id), err)
		return true, false
	}

	if existed {
		fmt.Printf("%s Event is already present in your calendar.\n", app.PrefixWarn)
	} else {
		fmt.Printf("%s Successfully added event to your calendar!\n", app.PrefixSuccess)
	}

	dayEvents, err := listEventsForDay(srv, event.StartParsed)
	if err == nil {
		printDayEvents(dayEvents, event.StartParsed, calEvent)
	} else {
		fmt.Printf("%s Failed to fetch daily schedule: %v\n", app.PrefixError, err)
	}

	return true, !existed
}

// PerformCalAddAll scans the inbox for messages with .ics attachments and adds them if not already present.
func PerformCalAddAll(config *Config, client mailclient.MailClient) error {
	localCfg := client.Config()

	matchedLabels, err := client.GetMatchingLabels(client.InboxFolder())
	if err != nil {
		return err
	}
	if len(matchedLabels) == 0 {
		return fmt.Errorf("no inbox label found")
	}

	srv, err := getCalendarService(config)
	if err != nil {
		return fmt.Errorf("failed to load calendar service: %w", err)
	}

	processedCount := 0
	addedCount := 0
	skippedCount := 0

	for _, matchedLabel := range matchedLabels {
		cacheDirName := cfg_g.SanitizeLabelForCache(matchedLabel)

		downloadedIDs, err := client.FetchAndDownloadEmails(matchedLabel, cacheDirName)
		if err != nil {
			return fmt.Errorf("error downloading emails for label %s: %w", matchedLabel, err)
		}

		fmt.Printf("%s Scanning label %q (%d messages)...\n", app.PrefixInfo, matchedLabel, len(downloadedIDs))

		for _, id := range downloadedIDs {
			processed, added := processCalendarMessage(srv, config, localCfg, id)
			if processed {
				processedCount++
				if added {
					addedCount++
				} else {
					skippedCount++
				}
			}
		}
	}

	fmt.Printf("%s Scan complete. Processed %d event(s). Added: %d, Skipped (existed): %d.\n", app.PrefixSuccess, processedCount, addedCount, skippedCount)
	return nil
}

// dayEventGroup groups calendar events by date string.
type dayEventGroup struct {
	dateStr string
	items   []*calendar.Event
}

// fetchCalendarEvents fetches all events between start and end times.
func fetchCalendarEvents(srv *calendar.Service, start, end time.Time) ([]*calendar.Event, error) {
	list, err := srv.Events.List("primary").
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar events: %w", err)
	}
	return list.Items, nil
}

// groupEventsByDay groups calendar events by their date string.
func groupEventsByDay(events []*calendar.Event, loc *time.Location) []dayEventGroup {
	var grouped []dayEventGroup
	for _, ev := range events {
		var evTime time.Time
		if ev.Start.Date != "" {
			evTime, _ = time.Parse("2006-01-02", ev.Start.Date)
		} else {
			evTime, _ = time.Parse(time.RFC3339, ev.Start.DateTime)
		}
		evTime = evTime.In(loc)
		dateStr := evTime.Format("Monday, 2006-01-02")

		foundIdx := -1
		for i, g := range grouped {
			if g.dateStr == dateStr {
				foundIdx = i
				break
			}
		}

		if foundIdx == -1 {
			grouped = append(grouped, dayEventGroup{
				dateStr: dateStr,
				items:   []*calendar.Event{ev},
			})
		} else {
			grouped[foundIdx].items = append(grouped[foundIdx].items, ev)
		}
	}
	return grouped
}

// printSingleWeekEvent prints a single event in the week view format.
func printSingleWeekEvent(ev *calendar.Event, loc *time.Location) {
	timeStr := ""
	if ev.Start.Date != "" {
		timeStr = app.ColorBoldYellow.Sprint("ALL DAY")
	} else {
		st, err1 := time.Parse(time.RFC3339, ev.Start.DateTime)
		et, err2 := time.Parse(time.RFC3339, ev.End.DateTime)
		if err1 == nil && err2 == nil {
			st = st.In(loc)
			et = et.In(loc)
			timeStr = fmt.Sprintf("%s - %s", st.Format("15:04"), et.Format("15:04"))
		} else {
			timeStr = "??:??"
		}
	}

	summary := ev.Summary
	if summary == "" {
		summary = "(No Title)"
	}

	locStr := ""
	if ev.Location != "" {
		locStr = fmt.Sprintf(" (%s)", ev.Location)
	}

	fmt.Printf("    [%s] %s%s\n", timeStr, summary, locStr)
}

// printWeekHeader prints the week view header.
func printWeekHeader(start, end time.Time) {
	app.ColorBoldCyan.Println("\n======================================================================")
	app.ColorBoldCyan.Printf("                CALENDAR EVENTS FOR %s to %s              \n", start.Format("2006-01-02"), end.AddDate(0, 0, -1).Format("2006-01-02"))
	app.ColorBoldCyan.Println("======================================================================")
}

// printWeekFooter prints the week view footer.
func printWeekFooter() {
	app.ColorBoldCyan.Println("\n======================================================================")
	fmt.Println()
}

// PerformCalendarWeek displays all events for the upcoming 7 days in the default calendar.
func PerformCalendarWeek(config *Config) error {
	srv, err := getCalendarService(config)
	if err != nil {
		return fmt.Errorf("failed to load calendar service: %w", err)
	}

	now := time.Now()
	loc := now.Location()
	year, month, day := now.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 7)

	events, err := fetchCalendarEvents(srv, start, end)
	if err != nil {
		return err
	}

	printWeekHeader(start, end)

	if len(events) == 0 {
		fmt.Println("  No events scheduled for the upcoming week.")
	} else {
		grouped := groupEventsByDay(events, loc)
		for _, g := range grouped {
			app.ColorBoldYellow.Printf("\n  %s:\n", g.dateStr)
			for _, ev := range g.items {
				printSingleWeekEvent(ev, loc)
			}
		}
	}

	printWeekFooter()
	return nil
}
