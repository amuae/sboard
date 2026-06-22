package handler

import (
	"testing"

	"github.com/sboard-go/sboard/internal/database"
)

// ========== getEffectiveDnsResolve tests ==========

func TestGetEffectiveDnsResolve(t *testing.T) {
	tests := []struct {
		name     string
		user     *database.ProxyUser
		server   *database.Server
		expected string
	}{
		{
			name: "user has explicit ipv4, server has ipv6 — uses user",
			user:     &database.ProxyUser{DnsResolve: "ipv4"},
			server:   &database.Server{DnsResolve: "ipv6"},
			expected: "ipv4",
		},
		{
			name: "user has default, server has ipv6 — falls back to server",
			user:     &database.ProxyUser{DnsResolve: "default"},
			server:   &database.Server{DnsResolve: "ipv6"},
			expected: "ipv6",
		},
		{
			name: "user has empty string, server has ipv4 — falls back to server",
			user:     &database.ProxyUser{DnsResolve: ""},
			server:   &database.Server{DnsResolve: "ipv4"},
			expected: "ipv4",
		},
		{
			name: "user has ipv6, server has ipv4 — uses user",
			user:     &database.ProxyUser{DnsResolve: "ipv6"},
			server:   &database.Server{DnsResolve: "ipv4"},
			expected: "ipv6",
		},
		{
			name: "both empty — returns empty from server",
			user:     &database.ProxyUser{DnsResolve: ""},
			server:   &database.Server{DnsResolve: ""},
			expected: "",
		},
		{
			name: "both default — returns server default",
			user:     &database.ProxyUser{DnsResolve: "default"},
			server:   &database.Server{DnsResolve: "default"},
			expected: "default",
		},
		{
			name: "user has ipv4, server empty — uses user",
			user:     &database.ProxyUser{DnsResolve: "ipv4"},
			server:   &database.Server{DnsResolve: ""},
			expected: "ipv4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEffectiveDnsResolve(tt.user, tt.server)
			if got != tt.expected {
				t.Errorf("getEffectiveDnsResolve() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ========== generateSS2022UserKeyForSublink tests ==========

func TestGenerateSS2022UserKeyForSublink(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"

	tests := []struct {
		name     string
		uuid     string
		method   string
		expected string
	}{
		{
			name:     "2022-blake3-aes-128-gcm produces 16-byte key",
			uuid:     uuid,
			method:   "2022-blake3-aes-128-gcm",
			expected: "o6nh7ZcyyrKIaBJ74A8c6Q==",
		},
		{
			name:     "2022-blake3-aes-256-gcm produces 32-byte key",
			uuid:     uuid,
			method:   "2022-blake3-aes-256-gcm",
			expected: "o6nh7ZcyyrKIaBJ74A8c6SGsrv3Vw7I6bp4Acr2cGjQ=",
		},
		{
			name:     "2022-blake3-chacha20-poly1305 produces 32-byte key",
			uuid:     uuid,
			method:   "2022-blake3-chacha20-poly1305",
			expected: "o6nh7ZcyyrKIaBJ74A8c6SGsrv3Vw7I6bp4Acr2cGjQ=",
		},
		{
			name:     "unknown method returns raw uuid",
			uuid:     uuid,
			method:   "aes-256-gcm",
			expected: uuid,
		},
		{
			name:     "empty method returns raw uuid",
			uuid:     uuid,
			method:   "",
			expected: uuid,
		},
		{
			name:     "different uuid produces different key",
			uuid:     "abcdef12-3456-7890-abcd-ef1234567890",
			method:   "2022-blake3-aes-128-gcm",
			expected: "sKRQWMS5THHQR9Gx5TL1yQ==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSS2022UserKeyForSublink(tt.uuid, tt.method)
			if got != tt.expected {
				t.Errorf("generateSS2022UserKeyForSublink(%q, %q) = %q, want %q",
					tt.uuid, tt.method, got, tt.expected)
			}
		})
	}
}

func TestGenerateSS2022UserKeyForSublink_Deterministic(t *testing.T) {
	uuid := "repeated-uuid-0000-0000-000000000000"
	first := generateSS2022UserKeyForSublink(uuid, "2022-blake3-aes-256-gcm")
	second := generateSS2022UserKeyForSublink(uuid, "2022-blake3-aes-256-gcm")
	if first != second {
		t.Errorf("same inputs should produce same key: %q != %q", first, second)
	}
}

// ========== matchRegex tests ==========

func TestMatchRegex(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		text        string
		expectedOk  bool
		expectError bool
	}{
		{
			name:       "empty pattern matches anything",
			pattern:    "",
			text:       "anything",
			expectedOk: true,
		},
		{
			name:       "simple substring match",
			pattern:    "hello",
			text:       "hello world",
			expectedOk: true,
		},
		{
			name:       "simple substring no match",
			pattern:    "goodbye",
			text:       "hello world",
			expectedOk: false,
		},
		{
			name:       "anchored pattern match",
			pattern:    "^[a-z]+$",
			text:       "hello",
			expectedOk: true,
		},
		{
			name:       "anchored pattern no match",
			pattern:    "^[a-z]+$",
			text:       "Hello123",
			expectedOk: false,
		},
		{
			name:        "invalid regex returns error",
			pattern:     "[unclosed",
			text:        "test",
			expectedOk:  false,
			expectError: true,
		},
		{
			name:       "domain-like pattern",
			pattern:    `\.google\.com$`,
			text:       "www.google.com",
			expectedOk: true,
		},
		{
			name:       "domain-like pattern no match",
			pattern:    `\.google\.com$`,
			text:       "www.github.com",
			expectedOk: false,
		},
		{
			name:       "IP regex match",
			pattern:    `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`,
			text:       "192.168.1.1",
			expectedOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := matchRegex(tt.pattern, tt.text)
			if tt.expectError {
				if err == nil {
					t.Errorf("matchRegex() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("matchRegex() unexpected error: %v", err)
				return
			}
			if ok != tt.expectedOk {
				t.Errorf("matchRegex() ok = %v, want %v", ok, tt.expectedOk)
			}
		})
	}
}

// ========== parseMultilineString tests ==========

func TestParseMultilineString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string returns nil",
			input:    "",
			expected: nil,
		},
		{
			name:     "single line",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "multiple lines",
			input:    "hello\nworld\nfoo",
			expected: []string{"hello", "world", "foo"},
		},
		{
			name:     "lines with trailing/leading whitespace",
			input:    "  hello  \n  world  \n  foo  ",
			expected: []string{"hello", "world", "foo"},
		},
		{
			name:     "blank lines are skipped",
			input:    "hello\n\n\nworld\n\nfoo",
			expected: []string{"hello", "world", "foo"},
		},
		{
			name:     "only blank lines yields nil",
			input:    "\n\n\n   \n  \n",
			expected: nil,
		},
		{
			name:     "domain list",
			input:    "google.com\nfacebook.com\ntwitter.com",
			expected: []string{"google.com", "facebook.com", "twitter.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMultilineString(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseMultilineString() len = %d, want %d (got=%v, want=%v)",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseMultilineString()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// ========== parsePortsString tests ==========

func TestParsePortsString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []interface{}
	}{
		{
			name:     "empty string returns nil",
			input:    "",
			expected: nil,
		},
		{
			name:     "single port",
			input:    "443",
			expected: []interface{}{443},
		},
		{
			name:     "multiple numeric ports",
			input:    "80,443,8080",
			expected: []interface{}{80, 443, 8080},
		},
		{
			name:     "port range preserved as string",
			input:    "8000-8100",
			expected: []interface{}{"8000-8100"},
		},
		{
			name:     "mixed ports and ranges",
			input:    "80,443,8000-8100,9090",
			expected: []interface{}{80, 443, "8000-8100", 9090},
		},
		{
			name:     "spaces around ports",
			input:    " 80 , 443 , 8080 ",
			expected: []interface{}{80, 443, 8080},
		},
		{
			name:     "empty parts are skipped",
			input:    "80,, ,443,",
			expected: []interface{}{80, 443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePortsString(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parsePortsString() len = %d, want %d (got=%v, want=%v)",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parsePortsString()[%d] = %v (type %T), want %v (type %T)",
						i, got[i], got[i], tt.expected[i], tt.expected[i])
				}
			}
		})
	}
}

// ========== parseDnsHijackString tests ==========

func TestParseDnsHijackString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string returns nil",
			input:    "",
			expected: nil,
		},
		{
			name:     "single domain",
			input:    "ads.example.com",
			expected: []string{"ads.example.com"},
		},
		{
			name:     "multiple domains",
			input:    "ads.example.com\ntracker.example.com",
			expected: []string{"ads.example.com", "tracker.example.com"},
		},
		{
			name:     "domains with whitespace",
			input:    "  ads.example.com  \n  tracker.example.com  ",
			expected: []string{"ads.example.com", "tracker.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDnsHijackString(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseDnsHijackString() len = %d, want %d (got=%v, want=%v)",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseDnsHijackString()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// ========== parseNameserverPolicy tests ==========

func TestParseNameserverPolicy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:     "empty string returns empty map",
			input:    "",
			expected: map[string]interface{}{},
		},
		{
			name:  "simple key-value",
			input: "example.com: 8.8.8.8",
			expected: map[string]interface{}{
				"example.com": []string{"8.8.8.8"},
			},
		},
		{
			name:  "rule-set with nameserver",
			input: "rule-set:cn_domain: 223.5.5.5",
			expected: map[string]interface{}{
				"rule-set:cn_domain": []string{"223.5.5.5"},
			},
		},
		{
			name:  "geosite with nameserver",
			input: "geosite:cn: 114.114.114.114",
			expected: map[string]interface{}{
				"geosite:cn": []string{"114.114.114.114"},
			},
		},
		{
			name:  "IPv6 nameserver",
			input: "example.com: 2001:4860:4860::8888",
			expected: map[string]interface{}{
				"example.com": []string{"2001:4860:4860::8888"},
			},
		},
		{
			name:  "multiple entries",
			input: "example.com: 8.8.8.8\nanother.com: 1.1.1.1",
			expected: map[string]interface{}{
				"example.com": []string{"8.8.8.8"},
				"another.com": []string{"1.1.1.1"},
			},
		},
		{
			name:     "blank lines are skipped",
			input:    "\n  \nexample.com: 8.8.8.8\n\n",
			expected: map[string]interface{}{"example.com": []string{"8.8.8.8"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNameserverPolicy(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseNameserverPolicy() len = %d, want %d (got=%v, want=%v)",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			for key, wantVal := range tt.expected {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("parseNameserverPolicy() missing key %q", key)
					continue
				}
				wantSlice := wantVal.([]string)
				gotSlice := gotVal.([]string)
				if len(wantSlice) != len(gotSlice) {
					t.Errorf("parseNameserverPolicy()[%q] len = %d, want %d",
						key, len(gotSlice), len(wantSlice))
					continue
				}
				for i := range wantSlice {
					if wantSlice[i] != gotSlice[i] {
						t.Errorf("parseNameserverPolicy()[%q][%d] = %q, want %q",
							key, i, gotSlice[i], wantSlice[i])
					}
				}
			}
		})
	}
}
