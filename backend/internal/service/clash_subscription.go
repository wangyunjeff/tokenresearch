package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	clashSidecarRootEnv        = "CLASH_SIDECAR_ROOT"
	clashDefaultSidecarRoot    = "/mnt/data/service_codex2/backend/data/proxy_sidecars"
	clashSidecarStartHTTPPort  = 17801
	clashSidecarStartSocksPort = 17901
	clashSidecarStartCtlPort   = 19091
	clashImportMaxBodyBytes    = 10 << 20
	clashImportMaxNodes        = 300
	clashImportDefaultUA       = "clash-verge/v2.5.1"
	clashImportBenchMinSeconds = 60
	clashImportBenchMaxSeconds = 300
	clashImportBenchDefaultTop = 10
	clashImportBenchMaxTop     = 50
	clashImportBenchTarget     = "https://api.openai.com/v1/models"
	clashImportBenchMihomoBin  = "/usr/bin/verge-mihomo"
	clashImportBenchClashBin   = "/home/ike/Downloads/clash/clash-linux-amd64-v1.18.0"
	clashImportBenchNodeKey    = "_service_codex2_benchmark_key"
)

var clashSidecarKeySanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// ClashSubscriptionImportInput controls the local Clash sidecar import.
type ClashSubscriptionImportInput struct {
	URL             string
	Config          string
	Filter          string
	StartSocksPort  int
	Limit           int
	Test            bool
	RestartSidecars bool
	Benchmark       ClashSubscriptionBenchmarkInput
}

type ClashSubscriptionImportResult struct {
	Total           int                           `json:"total"`
	Imported        int                           `json:"imported"`
	Created         int                           `json:"created"`
	Updated         int                           `json:"updated"`
	Skipped         int                           `json:"skipped"`
	Failed          int                           `json:"failed"`
	RestartRequired bool                          `json:"restart_required"`
	Restarted       bool                          `json:"restarted"`
	RestartError    string                        `json:"restart_error,omitempty"`
	RestartCommand  string                        `json:"restart_command"`
	Benchmark       *ClashSubscriptionBenchmark   `json:"benchmark,omitempty"`
	Items           []ClashSubscriptionImportItem `json:"items"`
}

type ClashSubscriptionImportItem struct {
	Name           string                    `json:"name"`
	Key            string                    `json:"key"`
	Type           string                    `json:"type"`
	ProxyID        int64                     `json:"proxy_id,omitempty"`
	Action         string                    `json:"action"`
	SocksURL       string                    `json:"socks_url,omitempty"`
	HTTPPort       int                       `json:"http_port,omitempty"`
	SocksPort      int                       `json:"socks_port,omitempty"`
	ControllerPort int                       `json:"controller_port,omitempty"`
	Error          string                    `json:"error,omitempty"`
	Test           *ProxyTestResult          `json:"test,omitempty"`
	Benchmark      *ClashNodeBenchmarkResult `json:"benchmark,omitempty"`
}

type clashSubscription struct {
	Proxies []map[string]any `yaml:"proxies"`
}

type ClashSubscriptionBenchmarkInput struct {
	Enabled             bool
	DurationSeconds     int
	TopN                int
	AllowedCountryCodes []string
}

type ClashSubscriptionBenchmark struct {
	Enabled             bool                       `json:"enabled"`
	DurationSeconds     int                        `json:"duration_seconds"`
	TopN                int                        `json:"top_n"`
	AllowedCountryCodes []string                   `json:"allowed_country_codes"`
	Target              string                     `json:"target"`
	StartedAt           int64                      `json:"started_at"`
	FinishedAt          int64                      `json:"finished_at"`
	Requested           int                        `json:"requested"`
	Eligible            int                        `json:"eligible"`
	Selected            int                        `json:"selected"`
	Skipped             int                        `json:"skipped"`
	Results             []ClashNodeBenchmarkResult `json:"results"`
}

type ClashNodeBenchmarkResult struct {
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Rank                int      `json:"rank,omitempty"`
	Selected            bool     `json:"selected"`
	Eligible            bool     `json:"eligible"`
	Reason              string   `json:"reason,omitempty"`
	Score               float64  `json:"score"`
	SuccessRate         float64  `json:"success_rate"`
	Samples             int      `json:"samples"`
	Successes           int      `json:"successes"`
	AvgLatencyMs        int64    `json:"avg_latency_ms,omitempty"`
	BestLatencyMs       int64    `json:"best_latency_ms,omitempty"`
	WorstLatencyMs      int64    `json:"worst_latency_ms,omitempty"`
	LastHTTPStatus      int      `json:"last_http_status,omitempty"`
	ExitIP              string   `json:"exit_ip,omitempty"`
	Country             string   `json:"country,omitempty"`
	CountryCode         string   `json:"country_code,omitempty"`
	AllowedCountryCodes []string `json:"allowed_country_codes,omitempty"`
	Error               string   `json:"error,omitempty"`
}

// ImportClashSubscription imports Clash nodes as local SOCKS sidecars.
//
// CUSTOM(service_codex2): this adapter intentionally lives outside normal proxy
// CRUD so upstream proxy schema changes have one small integration point.
func (s *adminServiceImpl) ImportClashSubscription(ctx context.Context, input ClashSubscriptionImportInput) (*ClashSubscriptionImportResult, error) {
	nodes, err := s.loadClashSubscription(ctx, input)
	if err != nil {
		return nil, err
	}

	filter := strings.TrimSpace(input.Filter)
	if filter != "" {
		filtered := nodes[:0]
		for _, node := range nodes {
			if strings.Contains(strings.ToLower(clashNodeName(node)), strings.ToLower(filter)) {
				filtered = append(filtered, node)
			}
		}
		nodes = filtered
	}

	if input.Limit > 0 && len(nodes) > input.Limit {
		nodes = nodes[:input.Limit]
	}
	if len(nodes) > clashImportMaxNodes {
		return nil, fmt.Errorf("too many clash nodes: %d exceeds limit %d", len(nodes), clashImportMaxNodes)
	}

	totalNodes := len(nodes)
	var benchmarkItems []ClashSubscriptionImportItem
	var benchmarkByKey map[string]*ClashNodeBenchmarkResult
	var benchmark *ClashSubscriptionBenchmark
	if input.Benchmark.Enabled {
		var err error
		nodes, benchmark, benchmarkItems, benchmarkByKey, err = s.benchmarkClashImportNodes(ctx, input.Benchmark, nodes)
		if err != nil {
			return nil, err
		}
	}

	result := &ClashSubscriptionImportResult{
		Total:           totalNodes,
		RestartRequired: len(nodes) > 0,
		RestartCommand:  "systemctl --user restart service-codex-proxy-sidecars.service",
		Benchmark:       benchmark,
		Skipped:         len(benchmarkItems),
	}
	if len(nodes) == 0 {
		result.Items = append(result.Items, benchmarkItems...)
		sortClashImportItems(result.Items)
		return result, nil
	}

	startSocksPort := input.StartSocksPort
	if startSocksPort <= 0 {
		startSocksPort = clashSidecarStartSocksPort
	}
	offset := startSocksPort - clashSidecarStartSocksPort
	firstHTTPPort := clashSidecarStartHTTPPort + offset
	firstControllerPort := clashSidecarStartCtlPort + offset
	if firstHTTPPort < 1 || firstHTTPPort > 65535 || firstControllerPort < 1 || firstControllerPort > 65535 {
		return nil, fmt.Errorf("start socks port must leave room for derived http/controller ports")
	}

	existingProxies, _, err := s.ListProxies(ctx, 1, 10000, "", "", "", "id", "asc")
	if err != nil {
		return nil, err
	}
	byPort := make(map[int]Proxy, len(existingProxies))
	usedPorts := make(map[int]bool, len(existingProxies)*3)
	for _, p := range existingProxies {
		usedPorts[p.Port] = true
		if p.Host == "127.0.0.1" && p.Protocol == "socks5h" {
			byPort[p.Port] = p
		}
	}
	sidecarRoot := clashSidecarRoot()
	existingSidecars := readClashSidecarSpecs(sidecarRoot, usedPorts)

	usedKeys := make(map[string]bool)
	for _, node := range nodes {
		name := clashNodeName(node)
		nodeType, _ := node["type"].(string)
		item := ClashSubscriptionImportItem{Name: name, Type: nodeType}
		if name == "" {
			item.Action = "failed"
			item.Error = "node has no name"
			result.Items = append(result.Items, item)
			result.Failed++
			continue
		}

		key := clashSidecarKey(name)
		baseKey := key
		for i := 2; usedKeys[key]; i++ {
			key = fmt.Sprintf("%s_%d", baseKey, i)
		}
		usedKeys[key] = true

		spec, hasExistingSidecar := existingSidecars[key]
		if !hasExistingSidecar {
			spec = clashSidecarSpecForKey(key, startSocksPort, usedPorts)
		}
		item.Key = spec.Key
		item.HTTPPort = spec.HTTPPort
		item.SocksPort = spec.SocksPort
		item.ControllerPort = spec.ControllerPort
		item.SocksURL = fmt.Sprintf("socks5h://127.0.0.1:%d", spec.SocksPort)
		if benchmarkByKey != nil {
			benchmarkKey := stringFromAny(node[clashImportBenchNodeKey])
			if benchmarkKey != "" {
				item.Benchmark = benchmarkByKey[benchmarkKey]
			}
			if item.Benchmark == nil {
				item.Benchmark = benchmarkByKey[clashBenchmarkNodeKey(node)]
			}
			delete(node, clashImportBenchNodeKey)
		}

		if err := writeClashSidecarConfig(sidecarRoot, spec, node); err != nil {
			item.Action = "failed"
			item.Error = err.Error()
			result.Items = append(result.Items, item)
			result.Failed++
			continue
		}

		if existing, ok := byPort[spec.SocksPort]; ok {
			updated, err := s.UpdateProxy(ctx, existing.ID, &UpdateProxyInput{
				Name:     name,
				Protocol: "socks5h",
				Host:     "127.0.0.1",
				Port:     spec.SocksPort,
				Status:   StatusActive,
			})
			if err != nil {
				item.Action = "failed"
				item.Error = err.Error()
				result.Items = append(result.Items, item)
				result.Failed++
				continue
			}
			item.ProxyID = updated.ID
			item.Action = "updated"
			result.Updated++
		} else {
			created, err := s.CreateProxy(ctx, &CreateProxyInput{
				Name:     name,
				Protocol: "socks5h",
				Host:     "127.0.0.1",
				Port:     spec.SocksPort,
			})
			if err != nil {
				item.Action = "failed"
				item.Error = err.Error()
				result.Items = append(result.Items, item)
				result.Failed++
				continue
			}
			item.ProxyID = created.ID
			item.Action = "created"
			result.Created++
		}

		result.Imported++
		result.Items = append(result.Items, item)
	}
	result.Items = append(result.Items, benchmarkItems...)

	if input.RestartSidecars && result.Imported > 0 {
		if err := restartClashSidecars(ctx); err != nil {
			result.RestartError = err.Error()
		} else {
			result.Restarted = true
			result.RestartRequired = false
		}
	}
	if input.Test {
		s.testClashImportItems(ctx, result)
	}

	sortClashImportItems(result.Items)
	return result, nil
}

func (s *adminServiceImpl) loadClashSubscription(ctx context.Context, input ClashSubscriptionImportInput) ([]map[string]any, error) {
	if strings.TrimSpace(input.Config) != "" {
		return parseClashSubscription([]byte(input.Config))
	}
	rawURL := strings.TrimSpace(input.URL)
	if rawURL == "" {
		return nil, errors.New("subscription url or config is required")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("subscription url must use http or https")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", clashImportDefaultUA)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription fetch failed: http %d", resp.StatusCode)
	}
	body, err := readLimited(resp, clashImportMaxBodyBytes)
	if err != nil {
		return nil, err
	}
	if decoded, ok := maybeDecodeBase64Subscription(body); ok {
		body = decoded
	}
	return parseClashSubscription(body)
}

func parseClashSubscription(body []byte) ([]map[string]any, error) {
	if decoded, ok := maybeDecodeBase64Subscription(body); ok {
		body = decoded
	}
	text := strings.TrimSpace(string(body))
	if looksLikeClashURIList(text) {
		return parseClashURIList(text)
	}

	var sub clashSubscription
	yamlErr := yaml.Unmarshal(body, &sub)
	if yamlErr != nil {
		return nil, yamlErr
	}
	if len(sub.Proxies) == 0 {
		var list []map[string]any
		if err := yaml.Unmarshal(body, &list); err == nil {
			sub.Proxies = list
		}
	}
	out := make([]map[string]any, 0, len(sub.Proxies))
	for _, node := range sub.Proxies {
		if clashNodeName(node) == "" {
			continue
		}
		out = append(out, node)
	}
	return out, nil
}

func maybeDecodeBase64Subscription(body []byte) ([]byte, bool) {
	text := strings.TrimSpace(string(body))
	if text == "" || strings.Contains(text, "proxies:") || looksLikeClashURIList(text) {
		return nil, false
	}
	decoded, err := decodeBase64SubscriptionText(text)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

func decodeBase64SubscriptionText(text string) ([]byte, error) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, text)
	return decodeFlexibleBase64(compact)
}

func looksLikeClashURIList(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return clashURIType(line) != ""
	}
	return false
}

func clashURIType(uri string) string {
	uri = strings.ToLower(strings.TrimSpace(uri))
	switch {
	case strings.HasPrefix(uri, "ss://"):
		return "ss"
	case strings.HasPrefix(uri, "ssr://"):
		return "ssr"
	case strings.HasPrefix(uri, "trojan://"):
		return "trojan"
	case strings.HasPrefix(uri, "vmess://"):
		return "vmess"
	case strings.HasPrefix(uri, "vless://"):
		return "vless"
	case strings.HasPrefix(uri, "hysteria://"):
		return "hysteria"
	case strings.HasPrefix(uri, "hysteria2://"), strings.HasPrefix(uri, "hy2://"):
		return "hysteria2"
	default:
		return ""
	}
}

func parseClashURIList(text string) ([]map[string]any, error) {
	nodes := make([]map[string]any, 0)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := parseClashURI(line)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, errors.New("no supported clash uri nodes found")
	}
	return nodes, nil
}

func parseClashURI(raw string) (map[string]any, error) {
	switch clashURIType(raw) {
	case "ss":
		return parseSSURI(raw)
	case "trojan":
		return parseTrojanURI(raw)
	case "vmess":
		return parseVMessURI(raw)
	case "vless":
		return parseVLESSURI(raw)
	default:
		return nil, fmt.Errorf("unsupported clash uri type")
	}
}

func parseSSURI(raw string) (map[string]any, error) {
	body := strings.TrimPrefix(strings.TrimSpace(raw), "ss://")
	body, fragment, _ := strings.Cut(body, "#")
	body, _, _ = strings.Cut(body, "?")

	var methodPass, hostPort string
	if left, right, ok := strings.Cut(body, "@"); ok {
		methodPass = decodeURIComponent(left)
		if !strings.Contains(methodPass, ":") {
			decoded, err := decodeFlexibleBase64(methodPass)
			if err == nil {
				methodPass = string(decoded)
			}
		}
		hostPort = decodeURIComponent(right)
	} else {
		decoded, err := decodeFlexibleBase64(body)
		if err != nil {
			return nil, err
		}
		methodPass, hostPort, ok = strings.Cut(string(decoded), "@")
		if !ok {
			return nil, errors.New("invalid ss uri")
		}
	}

	method, password, ok := strings.Cut(methodPass, ":")
	if !ok || strings.TrimSpace(method) == "" {
		return nil, errors.New("invalid ss method")
	}
	server, port, err := parseURIHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	name := uriNodeName(fragment, fmt.Sprintf("ss-%s-%d", server, port))
	return map[string]any{
		"name":     name,
		"type":     "ss",
		"server":   server,
		"port":     port,
		"cipher":   strings.TrimSpace(method),
		"password": password,
	}, nil
}

func parseTrojanURI(raw string) (map[string]any, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	server, port, err := parsedURIHostPort(parsed)
	if err != nil {
		return nil, err
	}
	password := ""
	if parsed.User != nil {
		password = parsed.User.Username()
	}
	if password == "" {
		return nil, errors.New("invalid trojan password")
	}
	q := parsed.Query()
	node := map[string]any{
		"name":     uriNodeName(parsed.Fragment, fmt.Sprintf("trojan-%s-%d", server, port)),
		"type":     "trojan",
		"server":   server,
		"port":     port,
		"password": password,
	}
	if sni := firstQuery(q, "sni", "peer", "servername"); sni != "" {
		node["sni"] = sni
	}
	addURITransportOptions(node, q)
	return node, nil
}

func parseVLESSURI(raw string) (map[string]any, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	server, port, err := parsedURIHostPort(parsed)
	if err != nil {
		return nil, err
	}
	uuid := ""
	if parsed.User != nil {
		uuid = parsed.User.Username()
	}
	if uuid == "" {
		return nil, errors.New("invalid vless uuid")
	}
	q := parsed.Query()
	node := map[string]any{
		"name":   uriNodeName(parsed.Fragment, fmt.Sprintf("vless-%s-%d", server, port)),
		"type":   "vless",
		"server": server,
		"port":   port,
		"uuid":   uuid,
	}
	if encryption := firstQuery(q, "encryption"); encryption != "" {
		node["encryption"] = encryption
	}
	if flow := firstQuery(q, "flow"); flow != "" {
		node["flow"] = flow
	}
	if security := strings.ToLower(firstQuery(q, "security")); security == "tls" || security == "reality" {
		node["tls"] = true
	}
	if servername := firstQuery(q, "sni", "servername"); servername != "" {
		node["servername"] = servername
	}
	if fingerprint := firstQuery(q, "fp"); fingerprint != "" {
		node["client-fingerprint"] = fingerprint
	}
	addURITransportOptions(node, q)
	return node, nil
}

func parseVMessURI(raw string) (map[string]any, error) {
	payload := strings.TrimPrefix(strings.TrimSpace(raw), "vmess://")
	decoded, err := decodeFlexibleBase64(payload)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(decoded, &cfg); err != nil {
		return nil, err
	}
	server := strings.TrimSpace(stringFromAny(cfg["add"]))
	port := yamlNumberToInt(cfg["port"])
	uuid := strings.TrimSpace(stringFromAny(cfg["id"]))
	if server == "" || port <= 0 || uuid == "" {
		return nil, errors.New("invalid vmess uri")
	}
	name := strings.TrimSpace(stringFromAny(cfg["ps"]))
	if name == "" {
		name = fmt.Sprintf("vmess-%s-%d", server, port)
	}
	node := map[string]any{
		"name":    name,
		"type":    "vmess",
		"server":  server,
		"port":    port,
		"uuid":    uuid,
		"alterId": yamlNumberToInt(cfg["aid"]),
		"cipher":  firstNonEmptyClashValue(stringFromAny(cfg["scy"]), "auto"),
	}
	if strings.EqualFold(stringFromAny(cfg["tls"]), "tls") || strings.EqualFold(stringFromAny(cfg["tls"]), "true") {
		node["tls"] = true
	}
	if sni := firstNonEmptyClashValue(stringFromAny(cfg["sni"]), stringFromAny(cfg["host"])); sni != "" {
		node["servername"] = sni
	}
	network := strings.TrimSpace(stringFromAny(cfg["net"]))
	switch network {
	case "ws":
		node["network"] = "ws"
		wsOpts := map[string]any{}
		if path := stringFromAny(cfg["path"]); path != "" {
			wsOpts["path"] = path
		}
		if host := stringFromAny(cfg["host"]); host != "" {
			wsOpts["headers"] = map[string]string{"Host": host}
		}
		if len(wsOpts) > 0 {
			node["ws-opts"] = wsOpts
		}
	case "grpc":
		node["network"] = "grpc"
		if serviceName := stringFromAny(cfg["path"]); serviceName != "" {
			node["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	case "tcp", "":
	default:
		node["network"] = network
	}
	return node, nil
}

func decodeFlexibleBase64(text string) ([]byte, error) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', ' ':
			return -1
		default:
			return r
		}
	}, text)
	var lastErr error
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(compact); err == nil {
			return decoded, nil
		} else {
			lastErr = err
		}
	}
	if len(compact)%4 != 0 {
		padded := compact + strings.Repeat("=", 4-len(compact)%4)
		for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
			if decoded, err := encoding.DecodeString(padded); err == nil {
				return decoded, nil
			} else {
				lastErr = err
			}
		}
	}
	return nil, lastErr
}

func parseURIHostPort(hostPort string) (string, int, error) {
	hostPort = strings.TrimSpace(strings.Trim(hostPort, "/"))
	if hostPort == "" {
		return "", 0, errors.New("missing host")
	}
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		host, portText, ok := strings.Cut(hostPort, ":")
		if !ok {
			return "", 0, err
		}
		host = strings.Trim(host, "[]")
		portText = strings.TrimRight(portText, "/")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, errors.New("invalid port")
	}
	return strings.TrimSpace(host), port, nil
}

func parsedURIHostPort(parsed *url.URL) (string, int, error) {
	server := strings.TrimSpace(parsed.Hostname())
	port, err := strconv.Atoi(parsed.Port())
	if server == "" || err != nil || port <= 0 || port > 65535 {
		return "", 0, errors.New("invalid host or port")
	}
	return server, port, nil
}

func addURITransportOptions(node map[string]any, q url.Values) {
	network := strings.ToLower(firstQuery(q, "type", "net", "network"))
	switch network {
	case "ws", "websocket":
		node["network"] = "ws"
		wsOpts := map[string]any{}
		if path := firstQuery(q, "path", "ws-path"); path != "" {
			wsOpts["path"] = path
		}
		if host := firstQuery(q, "host", "ws-host"); host != "" {
			wsOpts["headers"] = map[string]string{"Host": host}
		}
		if len(wsOpts) > 0 {
			node["ws-opts"] = wsOpts
		}
	case "grpc":
		node["network"] = "grpc"
		if serviceName := firstQuery(q, "serviceName", "service_name", "grpc-service-name"); serviceName != "" {
			node["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	case "", "tcp":
	default:
		node["network"] = network
	}
}

func uriNodeName(fragment, fallback string) string {
	name := strings.TrimSpace(decodeURIComponent(fragment))
	if name == "" {
		return fallback
	}
	return name
}

func decodeURIComponent(text string) string {
	decoded, err := url.QueryUnescape(text)
	if err != nil {
		return text
	}
	return decoded
}

func firstQuery(q url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(q.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func stringFromAny(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	case int, int64, float64:
		return strings.TrimSpace(fmt.Sprint(value))
	default:
		return ""
	}
}

func firstNonEmptyClashValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readLimited(resp *http.Response, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("subscription body exceeds %d bytes", limit)
	}
	return body, nil
}

type clashSidecarSpec struct {
	Key            string
	HTTPPort       int
	SocksPort      int
	ControllerPort int
}

func clashSidecarSpecForKey(key string, startSocksPort int, usedPorts map[int]bool) clashSidecarSpec {
	socksPort := nextFreePort(startSocksPort, usedPorts)
	offset := socksPort - clashSidecarStartSocksPort
	httpPort := clashSidecarStartHTTPPort + offset
	controllerPort := clashSidecarStartCtlPort + offset
	for usedPorts[httpPort] || usedPorts[controllerPort] {
		socksPort = nextFreePort(socksPort+1, usedPorts)
		offset = socksPort - clashSidecarStartSocksPort
		httpPort = clashSidecarStartHTTPPort + offset
		controllerPort = clashSidecarStartCtlPort + offset
	}
	usedPorts[socksPort] = true
	usedPorts[httpPort] = true
	usedPorts[controllerPort] = true
	return clashSidecarSpec{Key: key, HTTPPort: httpPort, SocksPort: socksPort, ControllerPort: controllerPort}
}

func nextFreePort(start int, used map[int]bool) int {
	for port := start; port <= 65535; port++ {
		if !used[port] {
			return port
		}
	}
	return start
}

func clashSidecarKey(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	key := clashSidecarKeySanitizer.ReplaceAllString(lower, "_")
	key = strings.Trim(key, "_")
	if len(key) > 40 {
		key = strings.Trim(key[:40], "_")
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:])[:8]
	if key == "" {
		key = "clash"
	}
	return fmt.Sprintf("%s_%s", key, suffix)
}

func clashNodeName(node map[string]any) string {
	name, _ := node["name"].(string)
	return strings.TrimSpace(name)
}

func clashSidecarRoot() string {
	if root := strings.TrimSpace(os.Getenv(clashSidecarRootEnv)); root != "" {
		return root
	}
	// CUSTOM(service_codex2): imported proxy rows are useless unless the web
	// import path and service-codex-proxy-sidecars.service write/read the same
	// directory. Keep this local path explicit so DATA_DIR preview runs cannot
	// silently generate sidecar configs under /tmp while the user service reads
	// backend/data/proxy_sidecars.
	return clashDefaultSidecarRoot
}

func readClashSidecarSpecs(root string, used map[int]bool) map[string]clashSidecarSpec {
	out := make(map[string]clashSidecarSpec)
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(root, entry.Name(), "config.yaml")
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var cfg map[string]any
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		httpPort := yamlNumberToInt(cfg["port"])
		socksPort := yamlNumberToInt(cfg["socks-port"])
		controllerPort := 0
		if httpPort > 0 {
			used[httpPort] = true
		}
		if socksPort > 0 {
			used[socksPort] = true
		}
		if raw, ok := cfg["external-controller"].(string); ok {
			if _, portText, ok := strings.Cut(raw, ":"); ok {
				if port, err := strconv.Atoi(portText); err == nil {
					used[port] = true
					controllerPort = port
				}
			}
		}
		if socksPort > 0 {
			out[entry.Name()] = clashSidecarSpec{
				Key:            entry.Name(),
				HTTPPort:       httpPort,
				SocksPort:      socksPort,
				ControllerPort: controllerPort,
			}
		}
	}
	return out
}

func writeClashSidecarConfig(root string, spec clashSidecarSpec, node map[string]any) error {
	sidecarDir := filepath.Join(root, spec.Key)
	if err := os.MkdirAll(filepath.Join(sidecarDir, "home"), 0755); err != nil {
		return err
	}
	name := clashNodeName(node)
	nodeConfig := copyClashNode(node)
	delete(nodeConfig, clashImportBenchNodeKey)
	cfg := map[string]any{
		"port":                spec.HTTPPort,
		"socks-port":          spec.SocksPort,
		"allow-lan":           false,
		"mode":                "Rule",
		"log-level":           "silent",
		"unified-delay":       true,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", spec.ControllerPort),
		"dns":                 map[string]any{"enable": false},
		"proxies":             []map[string]any{nodeConfig},
		"proxy-groups": []map[string]any{
			{
				"name":    "AUTO",
				"type":    "select",
				"proxies": []string{name},
			},
		},
		"rules": []string{"MATCH,AUTO"},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "config.yaml"), data, 0644); err != nil {
		return err
	}
	meta := map[string]any{
		"name":            name,
		"http_port":       spec.HTTPPort,
		"socks_port":      spec.SocksPort,
		"controller_port": spec.ControllerPort,
		"config_path":     filepath.Join(sidecarDir, "config.yaml"),
		"log_path":        filepath.Join(sidecarDir, "clash.log"),
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(sidecarDir, "meta.json"), metaData, 0644)
}

func yamlNumberToInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		out, _ := strconv.Atoi(strings.TrimSpace(n))
		return out
	default:
		return 0
	}
}

func sortClashImportItems(items []ClashSubscriptionImportItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Benchmark != nil || items[j].Benchmark != nil {
			leftRank := clashImportItemBenchmarkRank(items[i])
			rightRank := clashImportItemBenchmarkRank(items[j])
			if leftRank != rightRank {
				return leftRank < rightRank
			}
		}
		return items[i].SocksPort < items[j].SocksPort
	})
}

func clashImportItemBenchmarkRank(item ClashSubscriptionImportItem) int {
	if item.Benchmark == nil || item.Benchmark.Rank <= 0 {
		return 1 << 30
	}
	return item.Benchmark.Rank
}

func restartClashSidecars(ctx context.Context) error {
	restartCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(restartCtx, "systemctl", "--user", "restart", "service-codex-proxy-sidecars.service")
	out, err := cmd.CombinedOutput()
	if restartCtx.Err() != nil {
		return restartCtx.Err()
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("restart sidecars failed: %s", msg)
	}
	time.Sleep(1500 * time.Millisecond)
	return nil
}

func (s *adminServiceImpl) testClashImportItems(ctx context.Context, result *ClashSubscriptionImportResult) {
	if result == nil {
		return
	}
	for i := range result.Items {
		if result.Items[i].ProxyID <= 0 || (result.Items[i].Action != "created" && result.Items[i].Action != "updated") {
			continue
		}
		test, err := s.TestProxy(ctx, result.Items[i].ProxyID)
		if err != nil {
			result.Items[i].Test = &ProxyTestResult{Success: false, Message: err.Error()}
			continue
		}
		result.Items[i].Test = test
	}
}

type clashBenchmarkCandidate struct {
	Node      map[string]any
	BenchNode map[string]any
	Key       string
	Name      string
	BenchName string
	Type      string
}

func (s *adminServiceImpl) benchmarkClashImportNodes(
	ctx context.Context,
	input ClashSubscriptionBenchmarkInput,
	nodes []map[string]any,
) ([]map[string]any, *ClashSubscriptionBenchmark, []ClashSubscriptionImportItem, map[string]*ClashNodeBenchmarkResult, error) {
	benchInput := normalizeClashBenchmarkInput(input)
	summary := &ClashSubscriptionBenchmark{
		Enabled:             true,
		DurationSeconds:     benchInput.DurationSeconds,
		TopN:                benchInput.TopN,
		AllowedCountryCodes: benchInput.AllowedCountryCodes,
		Target:              clashImportBenchTarget,
		Requested:           len(nodes),
		StartedAt:           time.Now().Unix(),
	}
	if len(nodes) == 0 {
		summary.FinishedAt = time.Now().Unix()
		return nodes, summary, nil, nil, nil
	}
	clashBin, err := clashBenchmarkBinary()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	candidates := makeClashBenchmarkCandidates(nodes)
	results := make(map[string]*ClashNodeBenchmarkResult, len(candidates))
	for _, candidate := range candidates {
		results[candidate.Key] = &ClashNodeBenchmarkResult{
			Name:                candidate.Name,
			Type:                candidate.Type,
			AllowedCountryCodes: append([]string(nil), benchInput.AllowedCountryCodes...),
		}
	}

	tmpDir, err := os.MkdirTemp("", "service-codex2-clash-bench-*")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	httpPort, socksPort, controllerPort, err := clashBenchmarkPorts()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := writeClashBenchmarkConfig(configPath, candidates, httpPort, socksPort, controllerPort); err != nil {
		return nil, nil, nil, nil, err
	}

	benchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	logPath := filepath.Join(tmpDir, "clash.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.CommandContext(benchCtx, clashBin, "-d", filepath.Join(tmpDir, "home"), "-f", configPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("start clash benchmark core failed: %w", err)
	}
	defer stopClashBenchmarkProcess(cmd)

	controllerURL := fmt.Sprintf("http://127.0.0.1:%d", controllerPort)
	if err := waitForClashBenchmarkController(benchCtx, controllerURL, 20*time.Second); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("benchmark clash core did not become ready: %w; log: %s", err, readClashBenchmarkLogTail(logPath))
	}

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	deadline := time.Now().Add(time.Duration(benchInput.DurationSeconds) * time.Second)
	for time.Now().Before(deadline) {
		if err := benchCtx.Err(); err != nil {
			return nil, nil, nil, nil, err
		}
		for _, candidate := range candidates {
			if !time.Now().Before(deadline) {
				break
			}
			if err := selectClashBenchmarkProxy(benchCtx, controllerURL, candidate.BenchName); err != nil {
				result := results[candidate.Key]
				result.Error = err.Error()
				continue
			}
			if err := sleepClashBenchmark(benchCtx, 250*time.Millisecond); err != nil {
				break
			}
			result := results[candidate.Key]
			status, latency, err := probeClashBenchmarkOpenAI(benchCtx, proxyURL)
			result.Samples++
			result.LastHTTPStatus = status
			if err != nil {
				result.Error = err.Error()
				continue
			}
			result.Successes++
			recordClashBenchmarkLatency(result, latency)
			result.Error = ""
			if result.CountryCode == "" {
				s.fillClashBenchmarkExitInfo(benchCtx, result, proxyURL, candidate.Name)
			}
		}
	}

	finalizeClashBenchmarkResults(results, benchInput.AllowedCountryCodes)
	selectedKeys := selectClashBenchmarkWinners(results, benchInput.TopN)
	selectedSet := make(map[string]bool, len(selectedKeys))
	for rank, key := range selectedKeys {
		selectedSet[key] = true
		result := results[key]
		result.Selected = true
		result.Rank = rank + 1
	}

	selectedNodes := make([]map[string]any, 0, len(selectedKeys))
	candidateByKey := make(map[string]clashBenchmarkCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	for _, key := range selectedKeys {
		node := candidateByKey[key].Node
		node[clashImportBenchNodeKey] = key
		selectedNodes = append(selectedNodes, node)
	}

	skippedItems := make([]ClashSubscriptionImportItem, 0, len(candidates)-len(selectedNodes))
	byNodeKey := make(map[string]*ClashNodeBenchmarkResult, len(candidates)*2)
	summary.Results = make([]ClashNodeBenchmarkResult, 0, len(candidates))
	for _, candidate := range candidates {
		result := results[candidate.Key]
		byNodeKey[candidate.Key] = result
		byNodeKey[clashBenchmarkNodeKey(candidate.Node)] = result
		summary.Results = append(summary.Results, *result)
		if selectedSet[candidate.Key] {
			continue
		}
		reason := result.Reason
		if reason == "" {
			reason = "未进入 Top N"
		}
		skippedItems = append(skippedItems, ClashSubscriptionImportItem{
			Name:      candidate.Name,
			Key:       fmt.Sprintf("benchmark_%d", len(skippedItems)+1),
			Type:      candidate.Type,
			Action:    "skipped",
			Error:     reason,
			Benchmark: result,
		})
	}
	sort.SliceStable(summary.Results, func(i, j int) bool {
		left := summary.Results[i]
		right := summary.Results[j]
		if left.Selected != right.Selected {
			return left.Selected
		}
		if left.Rank != right.Rank {
			if left.Rank == 0 {
				return false
			}
			if right.Rank == 0 {
				return true
			}
			return left.Rank < right.Rank
		}
		if left.SuccessRate != right.SuccessRate {
			return left.SuccessRate > right.SuccessRate
		}
		return left.AvgLatencyMs < right.AvgLatencyMs
	})

	summary.FinishedAt = time.Now().Unix()
	for _, result := range results {
		if result.Eligible {
			summary.Eligible++
		}
		if result.Selected {
			summary.Selected++
		}
	}
	summary.Skipped = len(candidates) - summary.Selected
	return selectedNodes, summary, skippedItems, byNodeKey, nil
}

func recordClashBenchmarkLatency(result *ClashNodeBenchmarkResult, latency int64) {
	if result == nil || latency <= 0 || result.Successes <= 0 {
		return
	}
	if result.BestLatencyMs == 0 || latency < result.BestLatencyMs {
		result.BestLatencyMs = latency
	}
	if latency > result.WorstLatencyMs {
		result.WorstLatencyMs = latency
	}
	previousSuccesses := int64(result.Successes - 1)
	result.AvgLatencyMs = ((result.AvgLatencyMs * previousSuccesses) + latency) / int64(result.Successes)
}

func normalizeClashBenchmarkInput(input ClashSubscriptionBenchmarkInput) ClashSubscriptionBenchmarkInput {
	if input.DurationSeconds < clashImportBenchMinSeconds {
		input.DurationSeconds = clashImportBenchMinSeconds
	}
	if input.DurationSeconds > clashImportBenchMaxSeconds {
		input.DurationSeconds = clashImportBenchMaxSeconds
	}
	if input.TopN <= 0 {
		input.TopN = clashImportBenchDefaultTop
	}
	if input.TopN > clashImportBenchMaxTop {
		input.TopN = clashImportBenchMaxTop
	}
	if len(input.AllowedCountryCodes) == 0 {
		input.AllowedCountryCodes = []string{"US", "SG", "JP"}
	}
	seen := make(map[string]bool, len(input.AllowedCountryCodes))
	codes := make([]string, 0, len(input.AllowedCountryCodes))
	for _, code := range input.AllowedCountryCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		codes = []string{"US", "SG", "JP"}
	}
	input.AllowedCountryCodes = codes
	return input
}

func makeClashBenchmarkCandidates(nodes []map[string]any) []clashBenchmarkCandidate {
	nameCounts := map[string]int{}
	candidates := make([]clashBenchmarkCandidate, 0, len(nodes))
	for i, node := range nodes {
		name := clashNodeName(node)
		nameCounts[name]++
		benchName := name
		if nameCounts[name] > 1 {
			benchName = fmt.Sprintf("%s #%d", name, nameCounts[name])
		}
		benchNode := copyClashNode(node)
		benchNode["name"] = benchName
		nodeType, _ := node["type"].(string)
		key := clashBenchmarkNodeKeyWithIndex(node, i)
		candidates = append(candidates, clashBenchmarkCandidate{
			Node:      node,
			BenchNode: benchNode,
			Key:       key,
			Name:      name,
			BenchName: benchName,
			Type:      strings.TrimSpace(nodeType),
		})
	}
	return candidates
}

func copyClashNode(node map[string]any) map[string]any {
	out := make(map[string]any, len(node))
	for key, value := range node {
		out[key] = value
	}
	return out
}

func clashBenchmarkNodeKey(node map[string]any) string {
	return strings.Join([]string{
		clashNodeName(node),
		stringFromAny(node["type"]),
		stringFromAny(node["server"]),
		stringFromAny(node["port"]),
	}, "\x00")
}

func clashBenchmarkNodeKeyWithIndex(node map[string]any, index int) string {
	return fmt.Sprintf("%s\x00%d", clashBenchmarkNodeKey(node), index)
}

func clashBenchmarkBinary() (string, error) {
	for _, path := range []string{clashImportBenchMihomoBin, clashImportBenchClashBin} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("clash benchmark core not found: tried %s and %s", clashImportBenchMihomoBin, clashImportBenchClashBin)
}

func readClashBenchmarkLogTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unavailable"
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "empty"
	}
	const maxLen = 1600
	if len(text) > maxLen {
		text = text[len(text)-maxLen:]
	}
	return text
}

func clashBenchmarkPorts() (int, int, int, error) {
	used := map[int]bool{}
	httpPort, err := freeLocalTCPPort(used)
	if err != nil {
		return 0, 0, 0, err
	}
	socksPort, err := freeLocalTCPPort(used)
	if err != nil {
		return 0, 0, 0, err
	}
	controllerPort, err := freeLocalTCPPort(used)
	if err != nil {
		return 0, 0, 0, err
	}
	return httpPort, socksPort, controllerPort, nil
}

func freeLocalTCPPort(used map[int]bool) (int, error) {
	for i := 0; i < 20; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		if !used[port] {
			used[port] = true
			return port, nil
		}
	}
	return 0, errors.New("failed to allocate local benchmark ports")
}

func writeClashBenchmarkConfig(path string, candidates []clashBenchmarkCandidate, httpPort, socksPort, controllerPort int) error {
	names := make([]string, 0, len(candidates))
	proxies := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.BenchName)
		proxies = append(proxies, candidate.BenchNode)
	}
	cfg := map[string]any{
		"port":                httpPort,
		"socks-port":          socksPort,
		"allow-lan":           false,
		"mode":                "Rule",
		"log-level":           "silent",
		"unified-delay":       true,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", controllerPort),
		"dns":                 map[string]any{"enable": false},
		"proxies":             proxies,
		"proxy-groups": []map[string]any{
			{
				"name":    "BENCH",
				"type":    "select",
				"proxies": names,
			},
		},
		"rules": []string{"MATCH,BENCH"},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func waitForClashBenchmarkController(ctx context.Context, baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/version", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("controller returned http %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		if err := sleepClashBenchmark(ctx, 300*time.Millisecond); err != nil {
			return err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("controller did not become ready")
	}
	return lastErr
}

func selectClashBenchmarkProxy(ctx context.Context, baseURL, proxyName string) error {
	body, _ := json.Marshal(map[string]string{"name": proxyName})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/proxies/BENCH", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("select proxy failed: http %d", resp.StatusCode)
	}
	return nil
}

func probeClashBenchmarkOpenAI(ctx context.Context, proxyURL string) (int, int64, error) {
	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		return 0, 0, err
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyParsed),
		DialContext: (&net.Dialer{
			Timeout: 4 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 6 * time.Second,
		DisableKeepAlives:     true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   8 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clashImportBenchTarget, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, time.Since(start).Milliseconds(), err
	}
	defer func() { _ = resp.Body.Close() }()
	latency := time.Since(start).Milliseconds()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, proxyQualityMaxBodyBytes))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusOK {
		return resp.StatusCode, latency, nil
	}
	if resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(string(body)), "unsupported") {
		return resp.StatusCode, latency, fmt.Errorf("OpenAI region blocked: http %d", resp.StatusCode)
	}
	return resp.StatusCode, latency, fmt.Errorf("OpenAI returned http %d", resp.StatusCode)
}

func (s *adminServiceImpl) fillClashBenchmarkExitInfo(ctx context.Context, result *ClashNodeBenchmarkResult, proxyURL, nodeName string) {
	if result == nil {
		return
	}
	if s != nil && s.proxyProber != nil {
		probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		if exitInfo, _, err := s.proxyProber.ProbeProxy(probeCtx, proxyURL); err == nil && exitInfo != nil {
			result.ExitIP = exitInfo.IP
			result.Country = exitInfo.Country
			result.CountryCode = strings.ToUpper(strings.TrimSpace(exitInfo.CountryCode))
		}
	}
	if result.CountryCode == "" {
		result.CountryCode, result.Country = clashCountryFromName(nodeName)
	}
}

func clashCountryFromName(name string) (string, string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "singapore"), strings.Contains(lower, " sg"), strings.Contains(lower, "sg-"), strings.Contains(name, "新加坡"), strings.Contains(name, "狮城"):
		return "SG", "Singapore"
	case strings.Contains(lower, "japan"), strings.Contains(lower, " jp"), strings.Contains(lower, "jp-"), strings.Contains(lower, "tokyo"), strings.Contains(lower, "osaka"), strings.Contains(name, "日本"), strings.Contains(name, "东京"), strings.Contains(name, "大阪"):
		return "JP", "Japan"
	case strings.Contains(lower, "united states"), strings.Contains(lower, "usa"), strings.Contains(lower, " us"), strings.Contains(lower, "us-"), strings.Contains(name, "美国"), strings.Contains(name, "美國"):
		return "US", "United States"
	default:
		return "", ""
	}
}

func finalizeClashBenchmarkResults(results map[string]*ClashNodeBenchmarkResult, allowed []string) {
	allowedSet := make(map[string]bool, len(allowed))
	for _, code := range allowed {
		allowedSet[strings.ToUpper(strings.TrimSpace(code))] = true
	}
	for _, result := range results {
		if result.Samples > 0 {
			result.SuccessRate = float64(result.Successes) / float64(result.Samples)
		}
		if result.Successes == 0 {
			result.Reason = "OpenAI 不可达"
			continue
		}
		if result.CountryCode == "" {
			result.Reason = "无法确认出口地区"
			continue
		}
		if !allowedSet[strings.ToUpper(result.CountryCode)] {
			result.Reason = fmt.Sprintf("出口地区 %s 不在允许列表", result.CountryCode)
			continue
		}
		result.Eligible = true
		latencyPenalty := float64(result.AvgLatencyMs) / 1000
		result.Score = result.SuccessRate*100 - latencyPenalty
		if result.Score < 0 {
			result.Score = 0
		}
	}
}

func selectClashBenchmarkWinners(results map[string]*ClashNodeBenchmarkResult, topN int) []string {
	keys := make([]string, 0, len(results))
	for key, result := range results {
		if result.Eligible {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := results[keys[i]]
		right := results[keys[j]]
		if left.SuccessRate != right.SuccessRate {
			return left.SuccessRate > right.SuccessRate
		}
		if left.AvgLatencyMs != right.AvgLatencyMs {
			return left.AvgLatencyMs < right.AvgLatencyMs
		}
		if left.BestLatencyMs != right.BestLatencyMs {
			return left.BestLatencyMs < right.BestLatencyMs
		}
		return left.Name < right.Name
	})
	if len(keys) > topN {
		keys = keys[:topN]
	}
	return keys
}

func stopClashBenchmarkProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func sleepClashBenchmark(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
