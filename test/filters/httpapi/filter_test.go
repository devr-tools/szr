package httpapi_test

import (
	"strings"
	"testing"

	httpapifilter "github.com/devr-tools/szr/internal/filters/httpapi"
)

func TestSummarizeHTTPAPI(t *testing.T) {
	t.Parallel()

	jsonResponse := strings.Join([]string{
		"HTTP/2 200 OK",
		"content-type: application/json; charset=utf-8",
		"x-request-id: req_123",
		"",
		`{"id":"u_123","ok":true,"items":[{"name":"alpha","count":1}]}`,
	}, "\n")
	got := httpapifilter.SummarizeHTTPAPI(jsonResponse, 10)
	for _, want := range []string{
		"status=200 OK content-type=application/json",
		"root: object keys=3",
		`id="u_123"`,
		"items: array len=1 sample=object{name,count}",
		"ok=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in summarized API response:\n%s", want, got)
		}
	}

	plainText := strings.Join([]string{
		"HTTP/1.1 429 Too Many Requests",
		"content-type: text/plain",
		"location: https://api.example.test/retry",
		"",
		"rate limit exceeded",
		"retry later",
	}, "\n")
	got = httpapifilter.SummarizeHTTPAPI(plainText, 4)
	for _, want := range []string{
		"status=429 Too Many Requests content-type=text/plain location=https://api.example.test/retry",
		"rate limit exceeded",
		"retry later",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in plain-text API response:\n%s", want, got)
		}
	}
}

func TestHTTPAPIRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		"HTTP/1.1 429 Too Many Requests",
		"content-type: text/plain",
		"",
		"rate limit exceeded",
		"retry later",
		"contact support",
		"request id: req_123",
	}, "\n")

	kind, summary, requireRawCapture := httpapifilter.HTTPAPIRecoveryInfo(input, 3)
	if kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected HTTP API recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
