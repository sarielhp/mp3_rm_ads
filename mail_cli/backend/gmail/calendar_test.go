package gmail

import (
	"testing"
	"time"
)

func TestUnfoldICS(t *testing.T) {
	folded := "BEGIN:VEVENT\r\nSUMMARY:This is a very lo\r\n ng summary\r\nEND:VEVENT"
	got := unfoldICS(folded)
	want := "BEGIN:VEVENT\r\nSUMMARY:This is a very long summary\r\nEND:VEVENT"
	if got != want {
		t.Errorf("unfoldICS() = %q, want %q", got, want)
	}
}

func TestParseICSDateTime(t *testing.T) {
	tests := []struct {
		val     string
		tzid    string
		wantT   time.Time
		wantAll bool
		wantErr bool
	}{
		{
			val:     "20231024",
			tzid:    "",
			wantT:   time.Date(2023, 10, 24, 0, 0, 0, 0, time.UTC),
			wantAll: true,
			wantErr: false,
		},
		{
			val:     "20231024T170000Z",
			tzid:    "",
			wantT:   time.Date(2023, 10, 24, 17, 0, 0, 0, time.UTC),
			wantAll: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		gotT, gotAll, err := parseICSDateTime(tt.val, tt.tzid)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseICSDateTime(%q, %q) error = %v, wantErr %v", tt.val, tt.tzid, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if !gotT.Equal(tt.wantT) {
				t.Errorf("parseICSDateTime(%q, %q) gotT = %v, want %v", tt.val, tt.tzid, gotT, tt.wantT)
			}
			if gotAll != tt.wantAll {
				t.Errorf("parseICSDateTime(%q, %q) gotAll = %v, want %v", tt.val, tt.tzid, gotAll, tt.wantAll)
			}
		}
	}
}

func TestParseFirstICSEvent(t *testing.T) {
	ics := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:uid-1234
SUMMARY:Bastille Day Party
DESCRIPTION:Party time!
LOCATION:Paris
DTSTART:20230714T170000Z
DTEND:20230715T040000Z
END:VEVENT
END:VCALENDAR`

	event, err := parseFirstICSEvent(ics)
	if err != nil {
		t.Fatalf("parseFirstICSEvent failed: %v", err)
	}

	if event.UID != "uid-1234" {
		t.Errorf("got UID %q, want %q", event.UID, "uid-1234")
	}
	if event.Summary != "Bastille Day Party" {
		t.Errorf("got Summary %q, want %q", event.Summary, "Bastille Day Party")
	}
	if event.Description != "Party time!" {
		t.Errorf("got Description %q, want %q", event.Description, "Party time!")
	}
	if event.Location != "Paris" {
		t.Errorf("got Location %q, want %q", event.Location, "Paris")
	}
	if !event.StartParsed.Equal(time.Date(2023, 7, 14, 17, 0, 0, 0, time.UTC)) {
		t.Errorf("got Start %v, want %v", event.StartParsed, time.Date(2023, 7, 14, 17, 0, 0, 0, time.UTC))
	}
}
