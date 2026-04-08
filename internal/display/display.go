package display

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dwdankworth/wheresmybus/internal/api"
)

func PrintArrivals(arrivals []api.Arrival, stopID string, maxResults int) {
	if len(arrivals) == 0 {
		fmt.Printf("No upcoming arrivals for stop %s\n", stopID)
		return
	}

	sorted := append([]api.Arrival(nil), arrivals...)
	sort.Slice(sorted, func(i, j int) bool {
		return effectiveArrivalTime(sorted[i]) < effectiveArrivalTime(sorted[j])
	})

	sorted = collapseBunchedArrivals(sorted)

	if maxResults > 0 && len(sorted) > maxResults {
		sorted = sorted[:maxResults]
	}

	fmt.Printf("Arrivals for stop %s:\n", stopID)
	fmt.Printf("\n")

	rows := [][]string{
		{"ROUTE", "DESTINATION", "ETA", "STATUS"},
	}

	for _, arrival := range sorted {
		rows = append(rows, []string{
			arrival.RouteShortName,
			truncate(arrival.TripHeadsign, 30),
			formatETA(arrival),
			formatStatus(arrival),
		})
	}

	printTable(os.Stdout, rows)
}

func effectiveArrivalTime(arrival api.Arrival) int64 {
	if arrival.PredictedArrivalTime > 0 {
		return arrival.PredictedArrivalTime
	}

	return arrival.ScheduledArrivalTime
}

func formatETA(arrival api.Arrival) string {
	if !arrival.Predicted {
		return fmt.Sprintf("Scheduled %s", time.UnixMilli(arrival.ScheduledArrivalTime).Format("3:04 PM"))
	}

	until := time.Until(time.UnixMilli(effectiveArrivalTime(arrival)))
	if until < time.Minute {
		return "< 1 min"
	}

	minutes := int(until / time.Minute)
	if minutes == 1 {
		return "1 min"
	}

	return fmt.Sprintf("%d min", minutes)
}

func formatStatus(arrival api.Arrival) string {
	if arrival.Predicted {
		return fmt.Sprintf("%d stops away", arrival.NumberOfStopsAway)
	}

	return "Scheduled"
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}

	return string(runes[:max])
}

func printTable(output *os.File, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}

	border := tableBorder(widths)
	_, _ = fmt.Fprintln(output, border)
	for index, row := range rows {
		_, _ = fmt.Fprintln(output, tableRow(widths, row))
		if index == 0 {
			_, _ = fmt.Fprintln(output, border)
		}
	}
	_, _ = fmt.Fprintln(output, border)
}

func tableBorder(widths []int) string {
	var builder strings.Builder
	builder.WriteByte('+')
	for _, width := range widths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteByte('+')
	}
	return builder.String()
}

func tableRow(widths []int, row []string) string {
	var builder strings.Builder
	builder.WriteByte('|')
	for index, cell := range row {
		builder.WriteByte(' ')
		builder.WriteString(cell)
		builder.WriteString(strings.Repeat(" ", widths[index]-utf8.RuneCountInString(cell)+1))
		builder.WriteByte('|')
	}
	return builder.String()
}

const bunchThresholdMs = 60_000 // 60 seconds

// collapseBunchedArrivals removes arrivals on the same route+headsign whose
// predicted arrival times are within bunchThresholdMs of an already-kept
// arrival. Input must be sorted by effective arrival time.
func collapseBunchedArrivals(arrivals []api.Arrival) []api.Arrival {
	type routeKey struct {
		route    string
		headsign string
	}
	kept := make(map[routeKey][]int64)
	result := make([]api.Arrival, 0, len(arrivals))

	for _, a := range arrivals {
		key := routeKey{route: a.RouteShortName, headsign: a.TripHeadsign}
		arrTime := effectiveArrivalTime(a)

		bunched := false
		for _, prev := range kept[key] {
			if abs64(arrTime-prev) < bunchThresholdMs {
				bunched = true
				break
			}
		}

		if !bunched {
			kept[key] = append(kept[key], arrTime)
			result = append(result, a)
		}
	}
	return result
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
