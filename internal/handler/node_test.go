package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sboard-go/sboard/internal/database"
	"golang.org/x/crypto/curve25519"
)

func TestGenerateX25519KeyPair(t *testing.T) {
	privateKey, publicKey, err := generateX25519KeyPair()
	if err != nil {
		t.Fatalf("generateX25519KeyPair() error = %v", err)
	}

	privateBytes, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil || len(privateBytes) != 32 {
		t.Fatalf("private key is not a 32-byte raw base64url value: %q", privateKey)
	}
	publicBytes, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil || len(publicBytes) != 32 {
		t.Fatalf("public key is not a 32-byte raw base64url value: %q", publicKey)
	}

	var derived [32]byte
	var privateArray [32]byte
	copy(privateArray[:], privateBytes)
	curve25519.ScalarBaseMult(&derived, &privateArray)
	if !bytes.Equal(derived[:], publicBytes) {
		t.Fatal("generated public key does not match generated private key")
	}
}

func TestRealityInputValidation(t *testing.T) {
	validKey := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if !isValidRealityKey(validKey) {
		t.Fatal("expected a 32-byte raw base64url key to be valid")
	}
	for _, key := range []string{"", "too-short", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="} {
		if isValidRealityKey(key) {
			t.Fatalf("expected invalid Reality key %q to be rejected", key)
		}
	}
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "6469924b769d", valid: true},
		{value: "", valid: false},
		{value: "123", valid: false},
		{value: "zzzz", valid: false},
		{value: "1234567890abcdef12", valid: false},
	} {
		if got := isValidRealityShortID(test.value); got != test.valid {
			t.Errorf("isValidRealityShortID(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestHandleGenerateRealityKeysReturnsDataEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	(&Server{}).handleGenerateRealityKeys(context)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			PrivateKey string `json:"private_key"`
			PublicKey  string `json:"public_key"`
			ShortID    string `json:"short_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Data.PrivateKey == "" || response.Data.PublicKey == "" || response.Data.ShortID == "" {
		t.Fatalf("unexpected response: %s", recorder.Body.Bytes())
	}
}

func TestBuildSingBoxInboundIncludesRealityWithoutTlsFlag(t *testing.T) {
	node := &database.InboundNode{
		Tag:            "vless-reality",
		Protocol:       "vless",
		Listen:         "::",
		Port:           443,
		RealityEnabled: true,
		RealityServer:  "www.example.com",
		RealityPrivkey: "private-key",
		RealityShortId: "01234567",
	}

	inbound := buildSingBoxInbound(node, []NodeUser{{Name: "user", UUID: "uuid"}})
	data, err := json.Marshal(inbound)
	if err != nil {
		t.Fatalf("marshal inbound: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal inbound: %v", err)
	}
	tls, ok := config["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing from inbound: %s", data)
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality missing from inbound: %s", data)
	}
	if reality["private_key"] != node.RealityPrivkey {
		t.Errorf("private_key = %v, want %q", reality["private_key"], node.RealityPrivkey)
	}

	serverInbound := buildSingBoxInboundWithExtraUUIDs(
		node,
		[]NodeUserRelation{{UserID: 1, UUID: "uuid"}},
		map[uint]*database.ProxyUser{1: {ID: 1, Name: "user", UUID: "uuid"}},
		nil,
	)
	if serverInbound == nil {
		t.Fatal("buildSingBoxInboundWithExtraUUIDs() returned nil")
	}
	serverData, err := json.Marshal(serverInbound)
	if err != nil {
		t.Fatalf("marshal server inbound: %v", err)
	}
	var serverConfig map[string]interface{}
	if err := json.Unmarshal(serverData, &serverConfig); err != nil {
		t.Fatalf("unmarshal server inbound: %v", err)
	}
	serverTLS, ok := serverConfig["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing from server inbound: %s", serverData)
	}
	if _, ok := serverTLS["reality"].(map[string]interface{}); !ok {
		t.Fatalf("reality missing from server inbound: %s", serverData)
	}
}
