package wifi

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CurrentSSID returns the currently connected WiFi SSID, or an empty string when not connected.
func CurrentSSID() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return currentSSIDLinux()
	case "darwin":
		return currentSSIDDarwin()
	default:
		return "", fmt.Errorf("wifi detection not supported on %s", runtime.GOOS)
	}
}

func currentSSIDLinux() (string, error) {
	output, err := exec.Command("nmcli", "-t", "-f", "active,ssid", "dev", "wifi").Output()
	if err != nil {
		return "", nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "yes:") {
			continue
		}

		_, ssid, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		ssid = strings.TrimSpace(ssid)
		if ssid != "" {
			return ssid, nil
		}
	}

	return "", nil
}

func currentSSIDDarwin() (string, error) {
	output, err := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I").Output()
	if err != nil {
		return "", nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "SSID:") {
			continue
		}

		_, ssid, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}

		ssid = strings.TrimSpace(ssid)
		if ssid != "" {
			return ssid, nil
		}
	}

	return "", nil
}
