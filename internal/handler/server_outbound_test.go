package handler

import "testing"

func TestValidateOutboundRequest(t *testing.T) {
	tests := []struct {
		name  string
		input OutboundRequest
		valid bool
	}{
		{
			name:  "valid shadowsocks",
			input: OutboundRequest{Remark: " NAT ", Protocol: "shadowsocks", Host: " 203.0.113.10 ", Port: 443},
			valid: true,
		},
		{
			name:  "missing protocol",
			input: OutboundRequest{Remark: "nat", Host: "203.0.113.10", Port: 443},
		},
		{
			name:  "invalid port",
			input: OutboundRequest{Remark: "nat", Protocol: "socks5", Host: "203.0.113.10", Port: 0},
		},
		{
			name:  "reality requires vless",
			input: OutboundRequest{Remark: "nat", Protocol: "trojan", Host: "203.0.113.10", Port: 443, Reality: true},
		},
		{
			name:  "reality requires public key",
			input: OutboundRequest{Remark: "nat", Protocol: "vless", Host: "203.0.113.10", Port: 443, Reality: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutboundRequest(&tt.input)
			if (err == nil) != tt.valid {
				t.Fatalf("validateOutboundRequest() error = %v, valid = %v", err, tt.valid)
			}
		})
	}

	input := OutboundRequest{Remark: " nat ", Protocol: "ss", Host: " host.example ", Port: 443}
	if err := validateOutboundRequest(&input); err != nil {
		t.Fatalf("trimmed valid request rejected: %v", err)
	}
	if input.Remark != "nat" || input.Host != "host.example" {
		t.Fatalf("request was not normalized: %#v", input)
	}
}
