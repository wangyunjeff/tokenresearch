package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClashSubscriptionYAML(t *testing.T) {
	nodes, err := parseClashSubscription([]byte(`
proxies:
  - name: "US Test"
    type: ss
    server: example.com
    port: 443
  - name: "SG Test"
    type: trojan
    server: sg.example.com
    port: 443
`))
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "US Test", clashNodeName(nodes[0]))
	require.Equal(t, "trojan", nodes[1]["type"])
}

func TestParseClashSubscriptionBase64URIList(t *testing.T) {
	raw := "trojan://secret@example.com:443?sni=example.com#Example%20Trojan\n"
	encoded := "dHJvamFuOi8vc2VjcmV0QGV4YW1wbGUuY29tOjQ0Mz9zbmk9ZXhhbXBsZS5jb20jRXhhbXBsZSUyMFRyb2phbgo"

	nodes, err := parseClashSubscription([]byte(encoded))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Example Trojan", nodes[0]["name"])
	require.Equal(t, "trojan", nodes[0]["type"])
	require.Equal(t, "example.com", nodes[0]["server"])
	require.Equal(t, 443, nodes[0]["port"])
	require.Equal(t, "secret", nodes[0]["password"])

	nodes, err = parseClashSubscription([]byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Example Trojan", nodes[0]["name"])
}

func TestParseSSURI(t *testing.T) {
	nodes, err := parseClashSubscription([]byte("ss://YWVzLTI1Ni1nY206cGFzc0BleGFtcGxlLmNvbTo4NDQz#Example%20SS"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Example SS", nodes[0]["name"])
	require.Equal(t, "ss", nodes[0]["type"])
	require.Equal(t, "aes-256-gcm", nodes[0]["cipher"])
	require.Equal(t, "pass", nodes[0]["password"])
	require.Equal(t, "example.com", nodes[0]["server"])
	require.Equal(t, 8443, nodes[0]["port"])
}

func TestParseVMessAndVLESSURI(t *testing.T) {
	vmess := "vmess://eyJwcyI6IkV4YW1wbGUgVk1lc3MiLCJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjEyMzQ1Njc4LTEyMzQtMTIzNC0xMjM0LTEyMzQ1Njc4OTBhYiIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0bHMiOiJ0bHMiLCJwYXRoIjoiL3dzIiwiaG9zdCI6ImVkZ2UuZXhhbXBsZS5jb20ifQ"
	nodes, err := parseClashSubscription([]byte(vmess))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Example VMess", nodes[0]["name"])
	require.Equal(t, "vmess", nodes[0]["type"])
	require.Equal(t, "ws", nodes[0]["network"])
	require.Equal(t, true, nodes[0]["tls"])

	vless := "vless://12345678-1234-1234-1234-1234567890ab@example.com:443?security=tls&type=ws&sni=example.com&fp=chrome&path=%2Fws#Example%20VLESS"
	nodes, err = parseClashSubscription([]byte(vless))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Example VLESS", nodes[0]["name"])
	require.Equal(t, "vless", nodes[0]["type"])
	require.Equal(t, "example.com", nodes[0]["servername"])
	require.Equal(t, "chrome", nodes[0]["client-fingerprint"])
}

func TestWriteClashSidecarConfig(t *testing.T) {
	root := t.TempDir()
	spec := clashSidecarSpec{
		Key:            "us_test",
		HTTPPort:       17820,
		SocksPort:      17920,
		ControllerPort: 19110,
	}
	node := map[string]any{
		"name":   "US Test",
		"type":   "ss",
		"server": "example.com",
		"port":   443,
	}

	require.NoError(t, writeClashSidecarConfig(root, spec, node))
	configPath := filepath.Join(root, "us_test", "config.yaml")
	metaPath := filepath.Join(root, "us_test", "meta.json")
	config, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(config), "socks-port: 17920")
	require.Contains(t, string(config), "external-controller: 127.0.0.1:19110")
	require.FileExists(t, metaPath)
}
