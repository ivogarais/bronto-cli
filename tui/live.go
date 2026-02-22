package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ivogarais/bronto-cli/spec"
)

const defaultBrontoEndpoint = "https://api.eu.bronto.io"
const defaultBrontoConfigPath = ".bronto/config.json"

type brontoLiveClient struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

type liveQueryCache struct {
	metrics map[string]map[string]metricSeries
	logs    map[string][]map[string]any
}

var globalRateLimitState = struct {
	mu             sync.Mutex
	backoffUntil   time.Time
	consecutive429 int
}{
	backoffUntil: time.Time{},
}

type brontoRuntimeConfig struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"api_endpoint"`
}

func hydrateSpecWithLiveData(app *spec.AppSpec) (bool, error) {
	if app == nil {
		return false, nil
	}

	hasLive := false
	for _, ds := range app.Datasets {
		if ds.Live != nil {
			hasLive = true
			break
		}
	}
	if !hasLive {
		return false, nil
	}

	apiKey, endpoint, source, err := resolveBrontoCredentials()
	if err != nil {
		return true, err
	}

	c := brontoLiveClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	_ = source
	nowMS := time.Now().UTC().UnixMilli()
	cache := &liveQueryCache{
		metrics: map[string]map[string]metricSeries{},
		logs:    map[string][]map[string]any{},
	}

	for datasetID, ds := range app.Datasets {
		if ds.Live == nil {
			continue
		}
		if err := c.refreshDataset(context.Background(), datasetID, &ds, nowMS, cache); err != nil {
			return true, err
		}
		app.Datasets[datasetID] = ds
	}
	return true, nil
}

func resolveBrontoCredentials() (apiKey string, endpoint string, source string, err error) {
	apiKey = strings.TrimSpace(os.Getenv("BRONTO_API_KEY"))
	endpoint = strings.TrimSpace(os.Getenv("BRONTO_API_ENDPOINT"))
	if apiKey != "" {
		if endpoint == "" {
			endpoint = defaultBrontoEndpoint
		}
		return apiKey, strings.TrimRight(endpoint, "/"), "env", nil
	}

	configPath := strings.TrimSpace(os.Getenv("BRONTO_CONFIG_FILE"))
	if configPath == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			configPath = filepath.Join(home, defaultBrontoConfigPath)
		}
	}
	if configPath != "" {
		b, readErr := os.ReadFile(configPath)
		if readErr == nil {
			var cfg brontoRuntimeConfig
			if jsonErr := json.Unmarshal(b, &cfg); jsonErr == nil {
				cfgKey := strings.TrimSpace(cfg.APIKey)
				cfgEndpoint := strings.TrimSpace(cfg.Endpoint)
				if cfgKey != "" {
					if endpoint == "" {
						endpoint = cfgEndpoint
					}
					if endpoint == "" {
						endpoint = defaultBrontoEndpoint
					}
					return cfgKey, strings.TrimRight(endpoint, "/"), "config", nil
				}
			}
		}
	}

	return "", "", "", errors.New(
		"Missing Bronto credentials. Set BRONTO_API_KEY (and optional BRONTO_API_ENDPOINT), " +
			"or create ~/.bronto/config.json with {\"api_key\":\"...\",\"api_endpoint\":\"https://api.eu.bronto.io\"}",
	)
}

func (c brontoLiveClient) refreshDataset(
	ctx context.Context,
	datasetID string,
	ds *spec.DatasetSpec,
	nowMS int64,
	cache *liveQueryCache,
) error {
	live := ds.Live
	if live == nil {
		return nil
	}
	startMS := nowMS - int64(live.LookbackSec*1000)
	if live.Mode == "logs" {
		cacheKey := buildLiveQueryKey(ds.Kind, *live, startMS, nowMS)
		events, ok := cache.logs[cacheKey]
		var err error
		if !ok {
			events, err = c.searchLogs(ctx, *live, startMS, nowMS)
			if err != nil {
				return fmt.Errorf("dataset %q liveQuery logs failed: %w", datasetID, err)
			}
			cache.logs[cacheKey] = events
		}
		if err != nil {
			return fmt.Errorf("dataset %q liveQuery logs failed: %w", datasetID, err)
		}
		if len(events) == 0 {
			return nil
		}
		eventsCopy := make([]map[string]any, 0, len(events))
		for _, ev := range events {
			eventsCopy = append(eventsCopy, ev)
		}
		applyLogs(ds, eventsCopy)
		return nil
	}

	cacheKey := buildLiveQueryKey(ds.Kind, *live, startMS, nowMS)
	grouped, ok := cache.metrics[cacheKey]
	if !ok {
		resp, err := c.computeMetrics(ctx, *live, startMS, nowMS)
		if err != nil {
			return fmt.Errorf("dataset %q liveQuery metrics failed: %w", datasetID, err)
		}
		grouped = resp
		cache.metrics[cacheKey] = grouped
	}
	applyMetrics(ds, grouped)
	return nil
}

func (c brontoLiveClient) computeMetrics(
	ctx context.Context,
	live spec.LiveQuerySpec,
	startMS int64,
	endMS int64,
) (map[string]metricSeries, error) {
	payload := map[string]any{
		"from_ts":       startMS,
		"to_ts":         endMS,
		"where":         live.SearchFilter,
		"select":        live.MetricFunctions,
		"from":          live.LogIDs,
		"groups":        live.GroupByKeys,
		"num_of_slices": 10,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/search", strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BRONTO-API-KEY", c.apiKey)

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return parseMetricsResponse(raw), nil
}

func (c brontoLiveClient) searchLogs(
	ctx context.Context,
	live spec.LiveQuerySpec,
	startMS int64,
	endMS int64,
) ([]map[string]any, error) {
	params := url.Values{}
	for _, logID := range live.LogIDs {
		params.Add("from", logID)
	}
	params.Set("from_ts", strconv.FormatInt(startMS, 10))
	params.Set("to_ts", strconv.FormatInt(endMS, 10))
	params.Set("where", live.SearchFilter)
	params.Set("limit", strconv.Itoa(live.Limit))
	params.Add("select", "*")
	params.Add("select", "@raw")
	for _, key := range live.GroupByKeys {
		params.Add("groups", key)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-BRONTO-API-KEY", c.apiKey)

	resp, err := c.doRequestWithBackoff(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	eventsRaw, _ := raw["events"].([]any)
	events := make([]map[string]any, 0, len(eventsRaw))
	for _, item := range eventsRaw {
		ev, ok := item.(map[string]any)
		if !ok {
			continue
		}
		flat := flattenEvent(ev)
		events = append(events, flat)
	}
	return events, nil
}

func buildLiveQueryKey(kind string, live spec.LiveQuerySpec, startMS int64, endMS int64) string {
	return strings.Join(
		[]string{
			kind,
			live.Mode,
			strings.Join(live.LogIDs, ","),
			strings.Join(live.MetricFunctions, ","),
			live.SearchFilter,
			strings.Join(live.GroupByKeys, ","),
			strconv.Itoa(live.LookbackSec),
			strconv.Itoa(live.Limit),
			strconv.FormatInt(startMS, 10),
			strconv.FormatInt(endMS, 10),
		},
		"|",
	)
}

func (c brontoLiveClient) doRequestWithBackoff(req *http.Request) (*http.Response, error) {
	const max429Retries = 5
	for attempt := 0; attempt <= max429Retries; attempt++ {
		waitForGlobalRateWindow()
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			recordRateLimitSuccess()
			return resp, nil
		}
		retryAfter := resp.Header.Get("Retry-After")
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		delay := computeBackoffDelay(attempt, retryAfter)
		recordGlobalBackoff(delay)
		if attempt == max429Retries {
			return nil, fmt.Errorf("status=429 body=%s", strings.TrimSpace(string(body)))
		}
		time.Sleep(delay)
	}
	return nil, errors.New("unreachable retry state")
}

func waitForGlobalRateWindow() {
	globalRateLimitState.mu.Lock()
	until := globalRateLimitState.backoffUntil
	globalRateLimitState.mu.Unlock()
	if until.IsZero() {
		return
	}
	sleepFor := time.Until(until)
	if sleepFor > 0 {
		time.Sleep(sleepFor)
	}
}

func recordGlobalBackoff(delay time.Duration) {
	globalRateLimitState.mu.Lock()
	defer globalRateLimitState.mu.Unlock()
	globalRateLimitState.consecutive429++
	until := time.Now().Add(delay)
	if until.After(globalRateLimitState.backoffUntil) {
		globalRateLimitState.backoffUntil = until
	}
}

func recordRateLimitSuccess() {
	globalRateLimitState.mu.Lock()
	defer globalRateLimitState.mu.Unlock()
	globalRateLimitState.consecutive429 = 0
	if time.Now().After(globalRateLimitState.backoffUntil) {
		globalRateLimitState.backoffUntil = time.Time{}
	}
}

func computeBackoffDelay(attempt int, retryAfter string) time.Duration {
	if d, ok := parseRetryAfter(retryAfter); ok {
		if d < 500*time.Millisecond {
			d = 500 * time.Millisecond
		}
		return d
	}
	base := 800 * time.Millisecond
	exp := time.Duration(1<<minInt(attempt, 6)) * base
	if exp > 60*time.Second {
		exp = 60 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(exp / 3)))
	return exp + jitter
}

func parseRetryAfter(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(trimmed); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

type metricPoint struct {
	Timestamp int64
	Value     float64
	Count     float64
}

type metricSeries struct {
	Name   string
	Points []metricPoint
}

func parseMetricsResponse(raw map[string]any) map[string]metricSeries {
	out := map[string]metricSeries{}
	if groups, ok := raw["groups_series"].([]any); ok && len(groups) > 0 {
		for _, item := range groups {
			group, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(group["name"])
			points := parseMetricPoints(group["timeseries"])
			out[name] = metricSeries{Name: name, Points: points}
		}
		return out
	}

	totals, _ := raw["totals"].(map[string]any)
	points := parseMetricPoints(totals["timeseries"])
	out["total"] = metricSeries{Name: "total", Points: points}
	return out
}

func parseMetricPoints(raw any) []metricPoint {
	items, _ := raw.([]any)
	points := make([]metricPoint, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		points = append(points, metricPoint{
			Timestamp: asInt64(m["@timestamp"]),
			Value:     asFloat64(m["value"]),
			Count:     asFloat64(m["count"]),
		})
	}
	return points
}

func applyMetrics(ds *spec.DatasetSpec, grouped map[string]metricSeries) {
	switch ds.Kind {
	case "categorySeries":
		labels := make([]string, 0, len(grouped))
		values := make([]float64, 0, len(grouped))
		for name, series := range grouped {
			labels = append(labels, name)
			values = append(values, latestMetricValue(series.Points))
		}
		ds.Labels = labels
		ds.Values = values

	case "xySeries":
		xy := make([]spec.XYSeries, 0, len(grouped))
		for name, series := range grouped {
			points := make([]spec.XYPoint, 0, len(series.Points))
			for i, p := range series.Points {
				points = append(points, spec.XYPoint{X: float64(i + 1), Y: p.Value})
			}
			xy = append(xy, spec.XYSeries{Name: name, Points: points})
		}
		ds.XY = xy

	case "timeSeries":
		timeSeries := make([]spec.TimeSeries, 0, len(grouped))
		for name, series := range grouped {
			points := make([]spec.TimePoint, 0, len(series.Points))
			for _, p := range series.Points {
				t := time.UnixMilli(p.Timestamp).UTC().Format(time.RFC3339)
				points = append(points, spec.TimePoint{T: t, V: p.Value})
			}
			timeSeries = append(timeSeries, spec.TimeSeries{Name: name, Points: points})
		}
		ds.Time = timeSeries

	case "valueSeries":
		series, ok := grouped["total"]
		if !ok {
			for _, s := range grouped {
				series = s
				break
			}
		}
		values := make([]float64, 0, len(series.Points))
		for _, p := range series.Points {
			values = append(values, p.Value)
		}
		ds.Value = values

	case "table":
		rows := make([][]string, 0, len(grouped))
		for name, series := range grouped {
			row := map[string]any{
				"group": name,
				"value": formatFloat(latestMetricValue(series.Points)),
				"count": formatFloat(latestMetricCount(series.Points)),
			}
			rows = append(rows, rowByColumns(ds.Columns, row))
		}
		ds.Rows = rows
	}
}

func applyLogs(ds *spec.DatasetSpec, events []map[string]any) {
	switch ds.Kind {
	case "table":
		rows := make([][]string, 0, len(events))
		for _, event := range events {
			rows = append(rows, rowByColumns(ds.Columns, event))
		}
		ds.Rows = rows
	case "categorySeries":
		counts := map[string]float64{}
		for _, event := range events {
			label := asString(event["event.type"])
			if label == "" {
				label = "unknown"
			}
			counts[label] += 1
		}
		labels := make([]string, 0, len(counts))
		values := make([]float64, 0, len(counts))
		for label, value := range counts {
			labels = append(labels, label)
			values = append(values, value)
		}
		ds.Labels = labels
		ds.Values = values
	}
}

func flattenEvent(event map[string]any) map[string]any {
	out := map[string]any{}
	flattenMap("", event, out)
	if raw := asString(event["@raw"]); raw != "" {
		var parsed map[string]any
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			flattenMap("", parsed, out)
		}
	}
	return out
}

func flattenMap(prefix string, in map[string]any, out map[string]any) {
	for key, value := range in {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch typed := value.(type) {
		case map[string]any:
			flattenMap(fullKey, typed, out)
		default:
			out[fullKey] = value
		}
	}
}

func rowByColumns(columns []string, values map[string]any) []string {
	row := make([]string, 0, len(columns))
	for _, column := range columns {
		row = append(row, asString(values[column]))
	}
	return row
}

func latestMetricValue(points []metricPoint) float64 {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Value != 0 {
			return points[i].Value
		}
	}
	if len(points) == 0 {
		return 0
	}
	return points[len(points)-1].Value
}

func latestMetricCount(points []metricPoint) float64 {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Count != 0 {
			return points[i].Count
		}
	}
	if len(points) == 0 {
		return 0
	}
	return points[len(points)-1].Count
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func asString(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}

func asFloat64(v any) float64 {
	switch typed := v.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		f, _ := typed.Float64()
		return f
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		i, _ := typed.Int64()
		return i
	default:
		return 0
	}
}
