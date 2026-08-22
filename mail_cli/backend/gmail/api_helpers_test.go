package gmail_test

import (
	"mail_cli/backend/gmail"
	"testing"
)

func TestDecodeGmailRaw(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "Standard Base64URL no padding",
			input:   "SGVsbG8gV29ybGQ",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "Standard Base64URL with padding",
			input:   "SGVsbG8gV29ybGQ=",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "Standard Base64 with padding",
			input:   "SGVsbG8gV29ybGQ=",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "Base64 with newlines and spaces",
			input:   "SGVsbG8g\n V29ybGQ\r\n=",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "Base64 with URL characters",
			input:   "PDw_Pj4",
			want:    "<<?>>",
			wantErr: false,
		},
		{
			name:    "Base64 with standard slash character",
			input:   "PDw/Pj4=",
			want:    "<<?>>",
			wantErr: false,
		},
		{
			name:    "Invalid base64",
			input:   "!!!invalid!!!",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := gmail.DecodeGmailRaw(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("gmail.DecodeGmailRaw() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(gotBytes) != tt.want {
				t.Errorf("decodeGmailRaw() got = %q, want = %q", string(gotBytes), tt.want)
			}
		})
	}
}
