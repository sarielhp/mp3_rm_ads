package podcast

import (
	"testing"
	"time"
)

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "untitled"},
		{"   ", "untitled"},
		{"...", "untitled"},
		{"Dan Carlin's Hardcore History", "Dan_Carlins_Hardcore_History"},
		{"Empire - World History", "Empire_World_History"},
		{"In Our Time: Science", "In_Our_Time_Science"},
		{"המרקרים", "המרקרים"},
		{"West Virginia v. B.P.J..mp3", "West_Virginia_v_B_P_J_mp3"},
		{"Special Feature: Tom Hiddleston is the Oracle of Pompeii!", "Special_Feature_Tom_Hiddleston_is_the_Oracle_of_Pompeii"},
		{"What do we know about President Trump’s Venezuelan oil deal?", "What_do_we_know_about_President_Trumps_Venezuelan_oil_deal"},
		{"Episode #42 - The Fall of Rome", "Episode_42_The_Fall_of_Rome"},
	}

	for _, tc := range tests {
		got := SanitizeTitle(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeTitle(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFormatEpisodeFilename(t *testing.T) {
	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		date     time.Time
		ep       string
		title    string
		expected string
	}{
		{
			name:     "date, episode, and title",
			date:     date,
			ep:       "42",
			title:    "The Fall of Rome",
			expected: "2024-03-15_ep42_The_Fall_of_Rome.mp3",
		},
		{
			name:     "date and title only",
			date:     date,
			ep:       "",
			title:    "The Daily News",
			expected: "2024-03-15_The_Daily_News.mp3",
		},
		{
			name:     "episode and title only",
			date:     time.Time{},
			ep:       "105",
			title:    "Ancient Battles",
			expected: "ep105_Ancient_Battles.mp3",
		},
		{
			name:     "title already starts with ep42",
			date:     date,
			ep:       "42",
			title:    "ep42 - The Fall of Rome",
			expected: "2024-03-15_ep42_The_Fall_of_Rome.mp3",
		},
		{
			name:     "title already starts with Episode 42",
			date:     date,
			ep:       "42",
			title:    "Episode 42: The Fall of Rome",
			expected: "2024-03-15_Episode_42_The_Fall_of_Rome.mp3",
		},
	}

	for _, tc := range tests {
		got := FormatEpisodeFilename(tc.date, tc.ep, tc.title)
		if got != tc.expected {
			t.Errorf("[%s] FormatEpisodeFilename() = %q; want %q", tc.name, got, tc.expected)
		}
	}
}

func TestStripEpisodeFilenamePrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2024-03-15_ep42_The_Fall_of_Rome.mp3", "The_Fall_of_Rome"},
		{"2024-03-15_The_Daily_News", "The_Daily_News"},
		{"ep105_Ancient_Battles.mp3", "Ancient_Battles"},
		{"Plain_Title.mp3", "Plain_Title"},
		{"2024-03-15_ep42_The_Fall_of_Rome", "The_Fall_of_Rome"},
	}

	for _, tc := range tests {
		got := StripEpisodeFilenamePrefix(tc.input)
		if got != tc.expected {
			t.Errorf("StripEpisodeFilenamePrefix(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
