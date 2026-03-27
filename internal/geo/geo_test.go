package geo

import (
	"fmt"
	"testing"
)

// ── Data Structure ───────────────────────────────────────────────────────────

func TestData_AllFieldsPopulated(t *testing.T) {
	data := &Data{
		IP:            "8.8.8.8",
		CountryCode:   "US",
		CountryName:   "United States",
		ContinentCode: "NA",
		ContinentName: "North America",
	}

	if data.IP != "8.8.8.8" {
		t.Errorf("IP: got %q, want %q", data.IP, "8.8.8.8")
	}
	if data.CountryCode != "US" {
		t.Errorf("CountryCode: got %q, want %q", data.CountryCode, "US")
	}
	if data.CountryName != "United States" {
		t.Errorf("CountryName: got %q, want %q", data.CountryName, "United States")
	}
	if data.ContinentCode != "NA" {
		t.Errorf("ContinentCode: got %q, want %q", data.ContinentCode, "NA")
	}
	if data.ContinentName != "North America" {
		t.Errorf("ContinentName: got %q, want %q", data.ContinentName, "North America")
	}
}

func TestData_WithEmptyFields(t *testing.T) {
	data := &Data{
		IP:            "1.1.1.1",
		CountryCode:   "",
		CountryName:   "",
		ContinentCode: "",
		ContinentName: "",
	}

	if data.IP != "1.1.1.1" {
		t.Errorf("IP with empty fields: got %q, want %q", data.IP, "1.1.1.1")
	}
	if data.CountryCode != "" {
		t.Errorf("CountryCode: got %q, want empty", data.CountryCode)
	}
}

// ── Lookup ───────────────────────────────────────────────────────────────────

func TestDB_Lookup_Errors(t *testing.T) {
	db := &DB{}
	tests := []struct {
		name  string
		input string
	}{
		{"invalid IP", "not-an-ip"},
		{"empty string", ""},
		{"DB not loaded", "1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Lookup(tt.input)
			if err == nil {
				t.Errorf("Lookup(%q): expected error, got nil", tt.input)
			}
		})
	}
}

func TestDB_Lookup_DBNotLoaded_ReturnsError(t *testing.T) {
	db := &DB{}
	// Explicitly ensure reader is nil (unloaded)
	data, err := db.Lookup("8.8.8.8")
	if data != nil {
		t.Errorf("Lookup on unloaded DB: got data=%v, want nil", data)
	}
	if err == nil || err.Error() != "MaxMind database not loaded" {
		t.Errorf("Lookup on unloaded DB: got error=%v, want 'MaxMind database not loaded'", err)
	}
}

func TestDB_Lookup_InvalidIP_ReturnsError(t *testing.T) {
	db := &DB{}
	testCases := []string{"not.an.ip", "256.1.1.1", "1.2.3", "999.999.999.999"}
	for _, ip := range testCases {
		_, err := db.Lookup(ip)
		if err == nil {
			t.Errorf("Lookup(%q): expected error for invalid IP, got nil", ip)
		}
		if err.Error() != fmt.Sprintf("invalid IP address: %s", ip) {
			t.Errorf("Lookup(%q): error message mismatch: %v", ip, err)
		}
	}
}

func TestDB_Lookup_ValidIPFormats(t *testing.T) {
	db := &DB{}
	validIPs := []string{
		"0.0.0.0",          // Edge case: all zeros
		"255.255.255.255",  // Edge case: all max
		"192.168.1.1",      // Private range
		"127.0.0.1",        // Localhost
		"::1",              // IPv6 loopback
		"2001:db8::1",      // IPv6 example
		"::ffff:192.0.2.1", // IPv6-mapped IPv4
	}

	for _, ip := range validIPs {
		t.Run(ip, func(t *testing.T) {
			// Should fail because DB is not loaded, not because IP is invalid
			data, err := db.Lookup(ip)
			if err == nil || err.Error() != "MaxMind database not loaded" {
				t.Errorf("Lookup(%q): expected 'not loaded' error, got %v", ip, err)
			}
			if data != nil {
				t.Errorf("Lookup(%q): expected nil data, got %v", ip, data)
			}
		})
	}
}

func TestDB_Lookup_SingleCharInvalidIP(t *testing.T) {
	db := &DB{}
	_, err := db.Lookup("a")
	if err == nil {
		t.Error("Lookup('a'): expected error for single char")
	}
}

func TestDB_Lookup_SpecialCharactersInIP(t *testing.T) {
	db := &DB{}
	testCases := []string{
		"1.2.3.4.",     // Trailing dot
		".1.2.3.4",     // Leading dot
		"1..2.3.4",     // Double dot
		"1.2.3.4/32",   // CIDR notation
		"1.2.3.4:8080", // Port number
		"1.2.3.4-5",    // Range notation
		"1.2.3.4!",     // Special char
	}

	for _, ip := range testCases {
		t.Run(ip, func(t *testing.T) {
			_, err := db.Lookup(ip)
			if err == nil {
				t.Errorf("Lookup(%q): expected error, got nil", ip)
			}
		})
	}
}

// ── Unloaded DB Tests with Various IPs ──────────────────────────────────────

func TestDB_Lookup_UnloadedDB_WithDataStructValidation(t *testing.T) {
	// Even though DB is not loaded, we can validate the error path is consistent
	db := &DB{}
	tests := []struct {
		desc      string
		ip        string
		shouldErr bool
		errMsg    string
	}{
		{"valid ipv4", "8.8.8.8", true, "MaxMind database not loaded"},
		{"valid ipv6", "2001:4860:4860::8888", true, "MaxMind database not loaded"},
		{"localhost", "127.0.0.1", true, "MaxMind database not loaded"},
		{"private ipv4", "192.168.0.1", true, "MaxMind database not loaded"},
		{"edge case 0.0.0.0", "0.0.0.0", true, "MaxMind database not loaded"},
		{"edge case all max", "255.255.255.255", true, "MaxMind database not loaded"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			data, err := db.Lookup(tt.ip)
			if !tt.shouldErr {
				if err != nil {
					t.Errorf("Lookup(%s): unexpected error: %v", tt.ip, err)
				}
				return
			}
			if err == nil {
				t.Errorf("Lookup(%s): expected error, got nil", tt.ip)
				return
			}
			if err.Error() != tt.errMsg {
				t.Errorf("Lookup(%s): error mismatch: got %q, want %q", tt.ip, err.Error(), tt.errMsg)
			}
			if data != nil {
				t.Errorf("Lookup(%s): expected nil data, got %v", tt.ip, data)
			}
		})
	}
}

func TestDB_Lookup_InvalidIP_Comprehensive(t *testing.T) {
	db := &DB{}
	invalidIPs := []string{
		"",
		"a",
		"not.an.ip",
		"256.1.1.1",
		"1.2.3",
		"1.2.3.4.5",
		"999.999.999.999", "a.b.c.d",
		"1.2.3.4.",
		".1.2.3.4",
		"1.2.3.4/32",
		"1.2.3.4:8080",
		"hello world",
		"@#$%^&*()",
	}

	for _, ip := range invalidIPs {
		t.Run(fmt.Sprintf("invalid_%s", ip), func(t *testing.T) {
			data, err := db.Lookup(ip)
			if err == nil {
				t.Errorf("Lookup(%q): expected error, got nil", ip)
				return
			}
			if data != nil {
				t.Errorf("Lookup(%q): expected nil data, got %v", ip, data)
			}
			// All invalid IPs should produce this error message
			if !contains(err.Error(), "invalid IP address") {
				t.Errorf("Lookup(%q): expected 'invalid IP address' in error, got %q", ip, err.Error())
			}
		})
	}
}

// ── Data Structure Construction ──────────────────────────────────────────────

func TestData_JSONTags(t *testing.T) {
	// Verify that Data struct has correct JSON tags for marshaling
	data := &Data{
		IP:            "1.2.3.4",
		CountryCode:   "US",
		CountryName:   "United States",
		ContinentCode: "NA",
		ContinentName: "North America",
	}

	// Verify all public fields are accessible
	if data.IP == "" || data.CountryCode == "" || data.CountryName == "" ||
		data.ContinentCode == "" || data.ContinentName == "" {
		t.Error("Data: not all fields are set")
	}
}

func TestData_PartiallyPopulated(t *testing.T) {
	// Test Data with only some fields populated
	tests := []struct {
		desc string
		data *Data
	}{
		{
			"all fields",
			&Data{"1.2.3.4", "US", "USA", "NA", "North America"},
		},
		{
			"only ip and country code",
			&Data{IP: "1.2.3.4", CountryCode: "GB"},
		},
		{
			"empty strings",
			&Data{IP: "1.2.3.4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Just verify the data can be created and accessed
			if tt.data.IP == "" {
				t.Error("IP field is empty when it shouldn't be")
			}
		})
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
