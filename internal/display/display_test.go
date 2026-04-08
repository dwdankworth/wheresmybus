package display

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dwdankworth/wheresmybus/internal/api"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		max   int
		want  string
	}{
		{name: "empty string", value: "", max: 5, want: ""},
		{name: "short string", value: "bus", max: 10, want: "bus"},
		{name: "exact max length", value: "route", max: 5, want: "route"},
		{name: "over max length", value: "downtown", max: 4, want: "down"},
		{name: "unicode runes", value: "日本語", max: 2, want: "日本"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.value, tt.max)
			if got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.value, tt.max, got, tt.want)
			}
		})
	}
}

func TestEffectiveArrivalTime(t *testing.T) {
	tests := []struct {
		name    string
		arrival api.Arrival
		want    int64
	}{
		{
			name: "uses predicted arrival time when present",
			arrival: api.Arrival{
				PredictedArrivalTime: 1_700_000_001_000,
				ScheduledArrivalTime: 1_700_000_000_000,
			},
			want: 1_700_000_001_000,
		},
		{
			name: "falls back to scheduled arrival time",
			arrival: api.Arrival{
				PredictedArrivalTime: 0,
				ScheduledArrivalTime: 1_700_000_002_000,
			},
			want: 1_700_000_002_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveArrivalTime(tt.arrival)
			if got != tt.want {
				t.Fatalf("effectiveArrivalTime(%+v) = %d, want %d", tt.arrival, got, tt.want)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name    string
		arrival api.Arrival
		want    string
	}{
		{
			name: "predicted zero stops away",
			arrival: api.Arrival{
				Predicted:         true,
				NumberOfStopsAway: 0,
			},
			want: "0 stops away",
		},
		{
			name: "predicted one stop away",
			arrival: api.Arrival{
				Predicted:         true,
				NumberOfStopsAway: 1,
			},
			want: "1 stops away",
		},
		{
			name: "predicted multiple stops away",
			arrival: api.Arrival{
				Predicted:         true,
				NumberOfStopsAway: 5,
			},
			want: "5 stops away",
		},
		{
			name: "scheduled arrival",
			arrival: api.Arrival{
				Predicted: false,
			},
			want: "Scheduled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatus(tt.arrival)
			if got != tt.want {
				t.Fatalf("formatStatus(%+v) = %q, want %q", tt.arrival, got, tt.want)
			}
		})
	}
}

func TestFormatETA(t *testing.T) {
	scheduledTime := time.Date(2024, time.January, 2, 15, 4, 0, 0, time.Local)
	now := time.Now()

	tests := []struct {
		name    string
		arrival api.Arrival
		want    string
	}{
		{
			name: "scheduled time format",
			arrival: api.Arrival{
				Predicted:            false,
				ScheduledArrivalTime: scheduledTime.UnixMilli(),
			},
			want: "Scheduled 3:04 PM",
		},
		{
			name: "predicted less than one minute",
			arrival: api.Arrival{
				Predicted:            true,
				PredictedArrivalTime: now.Add(30 * time.Second).UnixMilli(),
			},
			want: "< 1 min",
		},
		{
			name: "predicted one minute",
			arrival: api.Arrival{
				Predicted:            true,
				PredictedArrivalTime: now.Add(time.Minute + 5*time.Second).UnixMilli(),
			},
			want: "1 min",
		},
		{
			name: "predicted multiple minutes",
			arrival: api.Arrival{
				Predicted:            true,
				PredictedArrivalTime: now.Add(5*time.Minute + 5*time.Second).UnixMilli(),
			},
			want: "5 min",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatETA(tt.arrival)
			if got != tt.want {
				t.Fatalf("formatETA(%+v) = %q, want %q", tt.arrival, got, tt.want)
			}
		})
	}
}

func TestPrintArrivals(t *testing.T) {
	now := time.Now()
	arrivals := []api.Arrival{
		{
			RouteShortName:       "LATE",
			TripHeadsign:         "Late bus",
			Predicted:            true,
			PredictedArrivalTime: now.Add(5 * time.Minute).UnixMilli(),
			ScheduledArrivalTime: now.Add(1 * time.Minute).UnixMilli(),
			NumberOfStopsAway:    5,
		},
		{
			RouteShortName:       "MIDDLE",
			TripHeadsign:         "Middle bus",
			Predicted:            false,
			ScheduledArrivalTime: now.Add(3 * time.Minute).UnixMilli(),
		},
		{
			RouteShortName:       "EARLY",
			TripHeadsign:         "Early bus",
			Predicted:            true,
			PredictedArrivalTime: now.Add(2 * time.Minute).UnixMilli(),
			ScheduledArrivalTime: now.Add(4 * time.Minute).UnixMilli(),
			NumberOfStopsAway:    1,
		},
	}

	tests := []struct {
		name       string
		arrivals   []api.Arrival
		stopID     string
		maxResults int
		assertions func(t *testing.T, output string)
	}{
		{
			name:       "empty arrivals list",
			arrivals:   nil,
			stopID:     "1234",
			maxResults: 0,
			assertions: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "No upcoming arrivals for stop 1234") {
					t.Fatalf("expected no arrivals message, got %q", output)
				}
			},
		},
		{
			name:       "normal list includes header and sorts by time",
			arrivals:   arrivals,
			stopID:     "5678",
			maxResults: 0,
			assertions: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "| ROUTE  | DESTINATION | ETA               | STATUS       |") {
					t.Fatalf("expected table header, got %q", output)
				}
				if !strings.Contains(output, "+--------+") {
					t.Fatalf("expected table border, got %q", output)
				}
				assertInOrder(t, output, "EARLY", "MIDDLE", "LATE")
			},
		},
		{
			name:       "max results truncates output",
			arrivals:   arrivals,
			stopID:     "5678",
			maxResults: 2,
			assertions: func(t *testing.T, output string) {
				t.Helper()
				assertInOrder(t, output, "EARLY", "MIDDLE")
				if strings.Contains(output, "LATE") {
					t.Fatalf("expected LATE to be omitted when maxResults=2, got %q", output)
				}
			},
		},
		{
			name:       "sorts using effective arrival time",
			arrivals:   arrivals,
			stopID:     "5678",
			maxResults: 0,
			assertions: func(t *testing.T, output string) {
				t.Helper()
				assertInOrder(t, output, "EARLY", "MIDDLE", "LATE")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				PrintArrivals(tt.arrivals, tt.stopID, tt.maxResults)
			})
			tt.assertions(t, output)
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return string(output)
}

func assertInOrder(t *testing.T, output string, values ...string) {
	t.Helper()

	lastIndex := -1
	for _, value := range values {
		index := strings.Index(output, value)
		if index == -1 {
			t.Fatalf("expected %q in output %q", value, output)
		}
		if index <= lastIndex {
			t.Fatalf("expected %q after previous value in output %q", value, output)
		}
		lastIndex = index
	}
}

func TestCollapseBunchedArrivals(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		arrivals []api.Arrival
		want     int
	}{
		{
			name:     "empty input",
			arrivals: nil,
			want:     0,
		},
		{
			name: "distinct routes preserved",
			arrivals: []api.Arrival{
				{RouteShortName: "44", TripHeadsign: "Downtown", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli()},
				{RouteShortName: "48", TripHeadsign: "Mt Baker", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli()},
			},
			want: 2,
		},
		{
			name: "bunched arrivals on same route collapsed",
			arrivals: []api.Arrival{
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli(), NumberOfStopsAway: 5},
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4*time.Minute + 1*time.Second).UnixMilli(), NumberOfStopsAway: 5},
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4*time.Minute + 2*time.Second).UnixMilli(), NumberOfStopsAway: 6},
			},
			want: 1,
		},
		{
			name: "same route with spread-out times kept",
			arrivals: []api.Arrival{
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli(), NumberOfStopsAway: 5},
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(20 * time.Minute).UnixMilli(), NumberOfStopsAway: 8},
			},
			want: 2,
		},
		{
			name: "different headsigns not collapsed",
			arrivals: []api.Arrival{
				{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli()},
				{RouteShortName: "67", TripHeadsign: "Northgate", Predicted: true, PredictedArrivalTime: now.Add(4*time.Minute + 5*time.Second).UnixMilli()},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseBunchedArrivals(tt.arrivals)
			if len(got) != tt.want {
				t.Fatalf("collapseBunchedArrivals returned %d arrivals, want %d", len(got), tt.want)
			}
		})
	}
}

func TestPrintArrivals_CollapsesBunchedBuses(t *testing.T) {
	now := time.Now()
	arrivals := []api.Arrival{
		{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4 * time.Minute).UnixMilli(), NumberOfStopsAway: 5},
		{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4*time.Minute + 1*time.Second).UnixMilli(), NumberOfStopsAway: 5},
		{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(4*time.Minute + 2*time.Second).UnixMilli(), NumberOfStopsAway: 4},
		{RouteShortName: "67", TripHeadsign: "Univ District", Predicted: true, PredictedArrivalTime: now.Add(20 * time.Minute).UnixMilli(), NumberOfStopsAway: 8},
	}

	output := captureStdout(t, func() {
		PrintArrivals(arrivals, "1_82235", 5)
	})

	count := strings.Count(output, "67")
	if count != 2 {
		t.Fatalf("expected 2 rows for route 67 (collapsed from 4), got %d\nOutput:\n%s", count, output)
	}
}
