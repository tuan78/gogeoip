package utils_test

import (
	"testing"
	"time"

	"github.com/tuan78/gogeoip/internal/utils"
)

func TestResolveInterval(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "empty string", input: "", wantErr: true},
		{name: "valid hours", input: "24h", want: 24 * time.Hour},
		{name: "valid minutes", input: "30m", want: 30 * time.Minute},
		{name: "valid seconds", input: "90s", want: 90 * time.Second},
		{name: "compound duration", input: "1h30m", want: 90 * time.Minute},
		{name: "negative duration", input: "-1h", want: -1 * time.Hour},
		{name: "invalid string", input: "invalid", wantErr: true},
		{name: "number without unit", input: "42", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ResolveInterval(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveInterval(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveInterval(%q): unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveInterval(%q): got %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
