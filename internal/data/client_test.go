package data

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(rt roundTripFunc) *Client {
	return NewClientWithHTTPClient(&http.Client{
		Timeout:   2 * time.Second,
		Transport: rt,
	})
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFetchPressureDelta_ParsesFirst24Hours(t *testing.T) {
	t.Parallel()

	// First 24 points have min=1000, max=1023 -> delta=23.
	// Extra points must be ignored.
	body := `{"hourly":{"surface_pressure":[1000,1001,1002,1003,1004,1005,1006,1007,1008,1009,1010,1011,1012,1013,1014,1015,1016,1017,1018,1019,1020,1021,1022,1023,500,2000]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.open-meteo.com" {
			t.Fatalf("unexpected host: %s", req.URL.Host)
		}
		if req.URL.Path != "/v1/forecast" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		q := req.URL.Query()
		if q.Get("hourly") != "surface_pressure" {
			t.Fatalf("unexpected hourly param: %q", q.Get("hourly"))
		}
		if q.Get("forecast_days") != "2" {
			t.Fatalf("unexpected forecast_days: %q", q.Get("forecast_days"))
		}
		return httpResponse(http.StatusOK, body), nil
	})

	delta, err := client.FetchPressureDelta(context.Background(), 48.1, 11.6)
	if err != nil {
		t.Fatalf("FetchPressureDelta returned error: %v", err)
	}
	if delta != 23 {
		t.Fatalf("FetchPressureDelta delta=%v, want 23", delta)
	}
}

func TestFetchPressureDelta_NotEnoughPoints(t *testing.T) {
	t.Parallel()

	body := `{"hourly":{"surface_pressure":[1000,1001,1002]}}`
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, body), nil
	})

	_, err := client.FetchPressureDelta(context.Background(), 48.1, 11.6)
	if err == nil || !strings.Contains(err.Error(), "not enough hourly pressure points") {
		t.Fatalf("expected not-enough-points error, got: %v", err)
	}
}

func TestFetchLatestKIndex_ParsesKnownKeysFromLastRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "k_index float",
			body: `[{"k_index":1.1},{"k_index":5.5}]`,
			want: 5.5,
		},
		{
			name: "kp_index string",
			body: `[{"kp_index":"3"},{"kp_index":"4.7"}]`,
			want: 4.7,
		},
		{
			name: "kp integer",
			body: `[{"kp":2},{"kp":6}]`,
			want: 6,
		},
		{
			name: "kIndex float",
			body: `[{"kIndex":1.0},{"kIndex":5.0}]`,
			want: 5,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newTestClient(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "services.swpc.noaa.gov" {
					t.Fatalf("unexpected host: %s", req.URL.Host)
				}
				return httpResponse(http.StatusOK, tc.body), nil
			})

			got, err := client.FetchLatestKIndex(context.Background())
			if err != nil {
				t.Fatalf("FetchLatestKIndex returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("FetchLatestKIndex got=%v, want=%v", got, tc.want)
			}
		})
	}
}

func TestFetchLatestKIndex_MissingField(t *testing.T) {
	t.Parallel()

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, `[{"x":1}]`), nil
	})

	_, err := client.FetchLatestKIndex(context.Background())
	if err == nil || !strings.Contains(err.Error(), "k-index field not found") {
		t.Fatalf("expected missing-k-index error, got: %v", err)
	}
}

func TestAnyToFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{name: "float64", input: 1.25, want: 1.25, ok: true},
		{name: "int", input: 7, want: 7, ok: true},
		{name: "string", input: "3.5", want: 3.5, ok: true},
		{name: "bad string", input: "abc", want: 0, ok: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := anyToFloat(tc.input)
			if ok != tc.ok {
				t.Fatalf("anyToFloat(%v) ok=%v, want %v", tc.input, ok, tc.ok)
			}
			if tc.ok && got != tc.want {
				t.Fatalf("anyToFloat(%v) got=%v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
