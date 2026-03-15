package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGetArrivalsFromURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
  "code": 200,
  "text": "OK",
  "data": {
    "entry": {
      "arrivalsAndDepartures": [
        {
          "routeShortName": "44",
          "tripHeadsign": "Downtown Seattle",
          "predictedArrivalTime": 1700000000000,
          "scheduledArrivalTime": 1700000060000,
          "numberOfStopsAway": 3,
          "predicted": true,
          "routeId": "1_100",
          "stopId": "1_75403",
          "tripId": "1_TRIP_A"
        },
        {
          "routeShortName": "48",
          "tripHeadsign": "Mount Baker",
          "predictedArrivalTime": 1700000100000,
          "scheduledArrivalTime": 1700000160000,
          "numberOfStopsAway": 5,
          "predicted": false,
          "routeId": "1_200",
          "stopId": "1_75403",
          "tripId": "1_TRIP_B"
        }
      ]
    }
  }
}`)
	}))
	defer server.Close()

	arrivals, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("GetArrivalsFromURL returned error: %v", err)
	}

	if len(arrivals) != 2 {
		t.Fatalf("expected 2 arrivals, got %d", len(arrivals))
	}

	if arrivals[0].RouteShortName != "44" || arrivals[0].TripHeadsign != "Downtown Seattle" || arrivals[0].PredictedArrivalTime != 1700000000000 || arrivals[0].ScheduledArrivalTime != 1700000060000 || arrivals[0].NumberOfStopsAway != 3 || !arrivals[0].Predicted || arrivals[0].RouteID != "1_100" || arrivals[0].StopID != "1_75403" || arrivals[0].TripID != "1_TRIP_A" {
		t.Fatalf("unexpected first arrival: %+v", arrivals[0])
	}

	if arrivals[1].RouteShortName != "48" || arrivals[1].TripHeadsign != "Mount Baker" || arrivals[1].PredictedArrivalTime != 1700000100000 || arrivals[1].ScheduledArrivalTime != 1700000160000 || arrivals[1].NumberOfStopsAway != 5 || arrivals[1].Predicted || arrivals[1].RouteID != "1_200" || arrivals[1].StopID != "1_75403" || arrivals[1].TripID != "1_TRIP_B" {
		t.Fatalf("unexpected second arrival: %+v", arrivals[1])
	}
}

func TestGetArrivalsFromURL_HTTPError404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":404,"text":"resource not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	_, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected error to contain 404, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), `{"code":404,"text":"resource not found"}`) {
		t.Fatalf("expected error to contain response body, got %q", err.Error())
	}
}

func TestGetArrivalsFromURL_HTTPError500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	_, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error to contain 500, got %q", err.Error())
	}
}

func TestGetArrivalsFromURL_HTTPError429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `{"code":429,"text":"rate limit exceeded"}`)
	}))
	defer server.Close()

	_, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "rate limited by OneBusAway") {
		t.Fatalf("expected rate-limit error, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), `{"code":429,"text":"rate limit exceeded"}`) {
		t.Fatalf("expected error to contain response body, got %q", err.Error())
	}
}

func TestGetArrivalsFromURL_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{invalid json`)
	}))
	defer server.Close()

	_, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Fatalf("expected unmarshal error, got %q", err.Error())
	}
}

func TestGetArrivalsFromURL_OBAErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":404,"text":"not found","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
	}))
	defer server.Close()

	_, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "onebusaway error 404: not found") {
		t.Fatalf("expected OBA-level error, got %q", err.Error())
	}
}

func TestGetArrivalsFromURL_EmptyArrivals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
	}))
	defer server.Close()

	arrivals, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(arrivals) != 0 {
		t.Fatalf("expected empty arrivals, got %d entries", len(arrivals))
	}
}

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func TestGetArrivals_URLConstruction(t *testing.T) {
	var receivedPath string
	var receivedKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
	}))
	defer server.Close()

	targetURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	originalClient := http.DefaultClient
	defer func() {
		http.DefaultClient = originalClient
	}()

	baseTransport := server.Client().Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	http.DefaultClient = &http.Client{Transport: rewriteTransport{base: baseTransport, target: targetURL}}

	const apiKey = "test-api-key"
	const stopID = "1_75403"

	arrivals, err := GetArrivals(apiKey, stopID)
	if err != nil {
		t.Fatalf("GetArrivals returned error: %v", err)
	}
	if len(arrivals) != 0 {
		t.Fatalf("expected empty arrivals, got %d", len(arrivals))
	}

	expectedPath := "/api/where/arrivals-and-departures-for-stop/1_75403.json"
	if receivedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, receivedPath)
	}

	if receivedKey != apiKey {
		t.Fatalf("expected key %q, got %q", apiKey, receivedKey)
	}
}

func TestGetArrivalsForStop_UsesExactStopIDWithoutResolution(t *testing.T) {
	var arrivalsPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/where/arrivals-and-departures-for-stop/1_75403.json":
			arrivalsPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	arrivals, resolvedStopID, err := getArrivals(server.Client(), server.URL, "test-api-key", "1_75403")
	if err != nil {
		t.Fatalf("getArrivals returned error: %v", err)
	}
	if len(arrivals) != 0 {
		t.Fatalf("expected no arrivals, got %d", len(arrivals))
	}
	if resolvedStopID != "1_75403" {
		t.Fatalf("resolved stop ID = %q, want %q", resolvedStopID, "1_75403")
	}
	if arrivalsPath != "/api/where/arrivals-and-departures-for-stop/1_75403.json" {
		t.Fatalf("arrivals path = %q, want exact stop path", arrivalsPath)
	}
}

func TestGetArrivalsForStop_ResolvesBareStopCode(t *testing.T) {
	var arrivalsPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/where/arrivals-and-departures-for-stop/1_71335.json":
			arrivalsPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[{"routeShortName":"542","tripHeadsign":"U-District Station","predictedArrivalTime":1700000000000,"scheduledArrivalTime":1700000060000,"numberOfStopsAway":4,"predicted":true,"routeId":"40_100511","stopId":"1_71335","tripId":"40_trip"}]}}}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	arrivals, resolvedStopID, err := getArrivals(server.Client(), server.URL, "test-api-key", "71335")
	if err != nil {
		t.Fatalf("getArrivals returned error: %v", err)
	}
	if resolvedStopID != "1_71335" {
		t.Fatalf("resolved stop ID = %q, want %q", resolvedStopID, "1_71335")
	}
	if len(arrivals) != 1 {
		t.Fatalf("expected 1 arrival, got %d", len(arrivals))
	}
	if arrivalsPath != "/api/where/arrivals-and-departures-for-stop/1_71335.json" {
		t.Fatalf("arrivals path = %q, want resolved stop path", arrivalsPath)
	}
}

func TestGetArrivalsForStop_BareStopCodeUsesPugetSoundPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/where/arrivals-and-departures-for-stop/1_25100.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	_, resolvedStopID, err := getArrivals(server.Client(), server.URL, "test-api-key", "25100")
	if err != nil {
		t.Fatalf("getArrivals returned error: %v", err)
	}
	if resolvedStopID != "1_25100" {
		t.Fatalf("resolved stop ID = %q, want %q", resolvedStopID, "1_25100")
	}
}

func TestGetArrivalsForStop_BareStopCodeNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/where/arrivals-and-departures-for-stop/1_71335.json":
			http.Error(w, `{"code":404,"text":"resource not found"}`, http.StatusNotFound)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	_, _, err := getArrivals(server.Client(), server.URL, "test-api-key", "71335")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected HTTP status 404") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `{"code":404,"text":"resource not found"}`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetArrivalsFromURL_ReadBodyError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       errReadCloser{},
			Header:     make(http.Header),
		}, nil
	})}

	_, err := GetArrivalsFromURL(client, "http://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "read response body") {
		t.Fatalf("expected read body error, got %q", err.Error())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct{}

func (errReadCloser) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (errReadCloser) Close() error {
	return nil
}

func TestDeduplicateArrivals(t *testing.T) {
	tests := []struct {
		name string
		in   []Arrival
		want int
	}{
		{
			name: "empty input",
			in:   nil,
			want: 0,
		},
		{
			name: "no duplicates",
			in: []Arrival{
				{TripID: "A", RouteShortName: "44"},
				{TripID: "B", RouteShortName: "48"},
			},
			want: 2,
		},
		{
			name: "duplicates collapsed",
			in: []Arrival{
				{TripID: "A", RouteShortName: "44"},
				{TripID: "A", RouteShortName: "44"},
				{TripID: "A", RouteShortName: "44"},
				{TripID: "B", RouteShortName: "48"},
			},
			want: 2,
		},
		{
			name: "empty tripId preserved",
			in: []Arrival{
				{TripID: "", RouteShortName: "44"},
				{TripID: "", RouteShortName: "48"},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateArrivals(tt.in)
			if len(got) != tt.want {
				t.Fatalf("deduplicateArrivals returned %d arrivals, want %d", len(got), tt.want)
			}
		})
	}
}

func TestDeduplicateArrivals_KeepsFirst(t *testing.T) {
	arrivals := []Arrival{
		{TripID: "A", NumberOfStopsAway: 5},
		{TripID: "A", NumberOfStopsAway: 3},
	}
	got := deduplicateArrivals(arrivals)
	if len(got) != 1 {
		t.Fatalf("expected 1 arrival, got %d", len(got))
	}
	if got[0].NumberOfStopsAway != 5 {
		t.Fatalf("expected first occurrence (5 stops away), got %d", got[0].NumberOfStopsAway)
	}
}

func TestGetArrivalsFromURL_Deduplicates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
  "code": 200,
  "text": "OK",
  "data": {
    "entry": {
      "arrivalsAndDepartures": [
        {"tripId": "1_T1", "routeShortName": "67", "tripHeadsign": "Univ District", "predictedArrivalTime": 1700000000000, "scheduledArrivalTime": 1700000060000, "numberOfStopsAway": 5, "predicted": true, "routeId": "1_100", "stopId": "1_82235"},
        {"tripId": "1_T1", "routeShortName": "67", "tripHeadsign": "Univ District", "predictedArrivalTime": 1700000000000, "scheduledArrivalTime": 1700000060000, "numberOfStopsAway": 5, "predicted": true, "routeId": "1_100", "stopId": "1_82235"},
        {"tripId": "1_T1", "routeShortName": "67", "tripHeadsign": "Univ District", "predictedArrivalTime": 1700000000000, "scheduledArrivalTime": 1700000060000, "numberOfStopsAway": 5, "predicted": true, "routeId": "1_100", "stopId": "1_82235"},
        {"tripId": "1_T2", "routeShortName": "67", "tripHeadsign": "Univ District", "predictedArrivalTime": 1700000120000, "scheduledArrivalTime": 1700000180000, "numberOfStopsAway": 7, "predicted": true, "routeId": "1_100", "stopId": "1_82235"}
      ]
    }
  }
}`)
	}))
	defer server.Close()

	arrivals, err := GetArrivalsFromURL(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("GetArrivalsFromURL returned error: %v", err)
	}

	if len(arrivals) != 2 {
		t.Fatalf("expected 2 arrivals after dedup, got %d", len(arrivals))
	}

	if arrivals[0].TripID != "1_T1" || arrivals[1].TripID != "1_T2" {
		t.Fatalf("unexpected trip IDs: %s, %s", arrivals[0].TripID, arrivals[1].TripID)
	}
}
