package display

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/dwdan/wheresmybus/internal/api"
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

	if maxResults > 0 && len(sorted) > maxResults {
		sorted = sorted[:maxResults]
	}

	fmt.Printf("Arrivals for stop %s:\n", stopID)
	fmt.Printf("\n")

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(writer, "%-8s\t%-30s\t%-18s\t%s\n", "ROUTE", "DESTINATION", "ETA", "STATUS")

	for _, arrival := range sorted {
		_, _ = fmt.Fprintf(writer, "%-8s\t%-30s\t%-18s\t%s\n",
			arrival.RouteShortName,
			truncate(arrival.TripHeadsign, 30),
			formatETA(arrival),
			formatStatus(arrival),
		)
	}

	_ = writer.Flush()
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
