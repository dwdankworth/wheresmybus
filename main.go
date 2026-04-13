package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/dwdankworth/wheresmybus/internal/api"
	"github.com/dwdankworth/wheresmybus/internal/config"
	"github.com/dwdankworth/wheresmybus/internal/display"
	"github.com/dwdankworth/wheresmybus/internal/updater"
	"github.com/dwdankworth/wheresmybus/internal/wifi"
)

const defaultMaxResults = 10

var version = "dev"

func main() {
	stop := flag.String("stop", "", "Query an explicit stop code or full stop ID")
	direction := flag.String("direction", "", "Which stop to query: 'home' or 'office'")
	maxResults := flag.Int("max-results", defaultMaxResults, "Maximum number of arrivals to show (0 for all)")
	printVersion := flag.Bool("version", false, "Print the version and exit")
	printConfigDir := flag.Bool("print-config-dir", false, "Print the platform-specific config directory and exit")
	update := flag.Bool("update", false, "Update to the latest release from GitHub")
	flag.Parse()

	if *printVersion {
		fmt.Println(versionString())
		return
	}

	if *printConfigDir {
		dir := config.ConfigDir()
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: could not determine config directory")
			os.Exit(1)
		}
		fmt.Println(dir)
		return
	}

	if *update {
		runUpdate(version)
		return
	}

	if err := validateFlags(*stop, *direction, *maxResults); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nRun the setup script or copy .env.example to the config directory.\nSee: wheresmybus -help or the README for details.\n", err)
		os.Exit(1)
	}

	stopRef, err := resolveStop(cfg, *stop, *direction, wifi.CurrentSSID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	arrivals, resolvedStopID, err := api.GetArrivalsForStop(cfg.APIKey, stopRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching arrivals: %v\n", err)
		os.Exit(1)
	}

	display.PrintArrivals(arrivals, resolvedStopID, *maxResults)
}

func versionString() string {
	v := version
	if v == "" {
		v = "dev"
	}
	return fmt.Sprintf("wheresmybus version %s", v)
}

func validateFlags(stop, direction string, maxResults int) error {
	if stop != "" && direction != "" {
		return fmt.Errorf("-stop and -direction cannot be used together")
	}
	if direction != "" && direction != "home" && direction != "office" {
		return fmt.Errorf("-direction must be 'home' or 'office'")
	}
	if maxResults < 0 {
		return fmt.Errorf("-max-results must be 0 or greater")
	}
	return nil
}

func resolveStop(cfg *config.Config, stop, direction string, detectSSID func() (string, error)) (string, error) {
	if stop != "" {
		return stop, nil
	}
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
		return "", fmt.Errorf("wifi detection failed: %w\nUse -direction home|office instead", err)
	}

	if ssid == "" {
		return "", fmt.Errorf("not connected to wifi\nUse: wheresmybus -direction home|office")
	}
	return "", fmt.Errorf("unknown wifi network %q\nUse: wheresmybus -direction home|office", ssid)
}

func runUpdate(currentVersion string) {
	result, err := updater.Check(nil, currentVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !result.UpdateAvailable {
		fmt.Printf("Already up to date (%s).\n", result.LatestVersion)
		return
	}

	fmt.Printf("Update available: %s → %s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Print("Apply update? [y/N] ")

	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		fmt.Println("\nUpdate cancelled.")
		return
	}
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("Update cancelled.")
		return
	}

	fmt.Print("Downloading...")
	if err := updater.Apply(nil, result.AssetURL, result.AssetName); err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\rUpdated to %s.   \n", result.LatestVersion)
}
