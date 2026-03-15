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
          "stopId": "1_75403"
        },
        {
          "routeShortName": "48",
          "tripHeadsign": "Mount Baker",
          "predictedArrivalTime": 1700000100000,
          "scheduledArrivalTime": 1700000160000,
          "numberOfStopsAway": 5,
          "predicted": false,
          "routeId": "1_200",
          "stopId": "1_75403"
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

	if arrivals[0].RouteShortName != "44" || arrivals[0].TripHeadsign != "Downtown Seattle" || arrivals[0].PredictedArrivalTime != 1700000000000 || arrivals[0].ScheduledArrivalTime != 1700000060000 || arrivals[0].NumberOfStopsAway != 3 || !arrivals[0].Predicted || arrivals[0].RouteID != "1_100" || arrivals[0].StopID != "1_75403" {
		t.Fatalf("unexpected first arrival: %+v", arrivals[0])
	}

	if arrivals[1].RouteShortName != "48" || arrivals[1].TripHeadsign != "Mount Baker" || arrivals[1].PredictedArrivalTime != 1700000100000 || arrivals[1].ScheduledArrivalTime != 1700000160000 || arrivals[1].NumberOfStopsAway != 5 || arrivals[1].Predicted || arrivals[1].RouteID != "1_200" || arrivals[1].StopID != "1_75403" {
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
		fmt.Fprint(w, `{"code":404,"text":"not found","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
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
		fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
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
		fmt.Fprint(w, `{"code":200,"text":"OK","data":{"entry":{"arrivalsAndDepartures":[]}}}`)
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
