package email

import "testing"

func TestIsSafeToAutoBlacklist(t *testing.T) {
	tests := []struct {
		name            string
		sender          string
		listUnsubscribe string
		score           float64
		want            bool
	}{
		{
			name:            "Safe to block campaign",
			sender:          "democrats@bounce.myngp.com",
			listUnsubscribe: "<mailto:unsubscribe@myngp.com>",
			score:           18.5,
			want:            true,
		},
		{
			name:            "Missing list unsubscribe",
			sender:          "democrats@bounce.myngp.com",
			listUnsubscribe: "",
			score:           18.5,
			want:            false,
		},
		{
			name:            "Low score",
			sender:          "democrats@bounce.myngp.com",
			listUnsubscribe: "<mailto:unsubscribe@myngp.com>",
			score:           12.0,
			want:            false,
		},
		{
			name:            "Public provider gmail",
			sender:          "spam-bot@gmail.com",
			listUnsubscribe: "<mailto:unsubscribe@gmail.com>",
			score:           20.0,
			want:            false,
		},
		{
			name:            "Public provider yahoo",
			sender:          "campaign@yahoo.com",
			listUnsubscribe: "<mailto:unsubscribe@yahoo.com>",
			score:           20.0,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSafeToAutoBlacklist(tt.sender, tt.listUnsubscribe, tt.score)
			if got != tt.want {
				t.Errorf("IsSafeToAutoBlacklist(%q, %q, %f) = %v, want %v", tt.sender, tt.listUnsubscribe, tt.score, got, tt.want)
			}
		})
	}
}
