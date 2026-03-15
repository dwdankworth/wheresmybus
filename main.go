package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dwdan/wheresmybus/internal/api"
	"github.com/dwdan/wheresmybus/internal/config"
	"github.com/dwdan/wheresmybus/internal/display"
	"github.com/dwdan/wheresmybus/internal/wifi"
)

const maxResults = 5

func main() {
	direction := flag.String("direction", "", "Which stop to query: 'home' or 'office'")
	flag.Parse()

	if *direction != "" && *direction != "home" && *direction != "office" {
		fmt.Fprintf(os.Stderr, "Error: --direction must be 'home' or 'office'\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nCopy .env.example to .env and fill in your values.\n", err)
		os.Exit(1)
	}

	stopID, err := resolveStop(cfg, *direction, wifi.CurrentSSID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	arrivals, err := api.GetArrivals(cfg.APIKey, stopID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching arrivals: %v\n", err)
		os.Exit(1)
	}

	display.PrintArrivals(arrivals, stopID, maxResults)
}

func resolveStop(cfg *config.Config, direction string, detectSSID func() (string, error)) (string, error) {
	if direction == "home" {
		return cfg.HomeStopID, nil
	}
	if direction == "office" {
		return cfg.OfficeStopID, nil
	}

	// Auto-detect from wifi
	ssid, err := detectSSID()
	if err != nil {
		return "", fmt.Errorf("wifi detection failed: %w\nUse --direction home|office instead", err)
	}

	switch ssid {
	case cfg.HomeWifi:
		return cfg.OfficeStopID, nil // At home → heading to office
	case cfg.OfficeWifi:
		return cfg.HomeStopID, nil // At office → heading home
	default:
		if ssid == "" {
			return "", fmt.Errorf("not connected to wifi\nUse: wheresmybus --direction home|office")
		}
		return "", fmt.Errorf("unknown wifi network %q\nUse: wheresmybus --direction home|office", ssid)
	}
}
