package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dwdankworth/wheresmybus/internal/api"
	"github.com/dwdankworth/wheresmybus/internal/config"
	"github.com/dwdankworth/wheresmybus/internal/display"
	"github.com/dwdankworth/wheresmybus/internal/wifi"
)

const maxResults = 5

func main() {
	direction := flag.String("direction", "", "Which stop to query: 'home' or 'office'")
	printConfigDir := flag.Bool("print-config-dir", false, "Print the platform-specific config directory and exit")
	flag.Parse()

	if *printConfigDir {
		dir := config.ConfigDir()
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: could not determine config directory")
			os.Exit(1)
		}
		fmt.Println(dir)
		return
	}

	if *direction != "" && *direction != "home" && *direction != "office" {
		fmt.Fprintf(os.Stderr, "Error: --direction must be 'home' or 'office'\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nRun the setup script or copy .env.example to the config directory.\nSee: wheresmybus --help or the README for details.\n", err)
		os.Exit(1)
	}

	stopRef, err := resolveStop(cfg, *direction, wifi.CurrentSSID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	arrivals, resolvedStopID, err := api.GetArrivalsForStop(cfg.APIKey, stopRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching arrivals: %v\n", err)
		os.Exit(1)
	}

	display.PrintArrivals(arrivals, resolvedStopID, maxResults)
}

func resolveStop(cfg *config.Config, direction string, detectSSID func() (string, error)) (string, error) {
	if direction == "home" {
		return cfg.OfficeStopID, nil // Going home → catch bus at office stop
	}
	if direction == "office" {
		return cfg.HomeStopID, nil // Going to office → catch bus at home stop
	}

	// Auto-detect from wifi
	ssid, err := detectSSID()
	if err == nil {
		switch ssid {
		case cfg.HomeWifi:
			return cfg.HomeStopID, nil // At home → show nearby stop
		case cfg.OfficeWifi:
			return cfg.OfficeStopID, nil // At office → show nearby stop
		}
	}

	if cfg.DefaultLocation != "" {
		switch cfg.DefaultLocation {
		case "home":
			return cfg.HomeStopID, nil
		case "office":
			return cfg.OfficeStopID, nil
		}
	}

	if err != nil {
		return "", fmt.Errorf("wifi detection failed: %w\nUse --direction home|office instead", err)
	}

	if ssid == "" {
		return "", fmt.Errorf("not connected to wifi\nUse: wheresmybus --direction home|office")
	}
	return "", fmt.Errorf("unknown wifi network %q\nUse: wheresmybus --direction home|office", ssid)
}
