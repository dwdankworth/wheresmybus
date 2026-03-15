package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

const baseURL = "https://api.pugetsound.onebusaway.org"

type Arrival struct {
	RouteShortName       string `json:"routeShortName"`
	TripHeadsign         string `json:"tripHeadsign"`
	PredictedArrivalTime int64  `json:"predictedArrivalTime"`
	ScheduledArrivalTime int64  `json:"scheduledArrivalTime"`
	NumberOfStopsAway    int    `json:"numberOfStopsAway"`
	Predicted            bool   `json:"predicted"`
	RouteID              string `json:"routeId"`
	StopID               string `json:"stopId"`
	TripID               string `json:"tripId"`
}

type obaResponse struct {
	Code int    `json:"code"`
	Text string `json:"text"`
	Data struct {
		Entry struct {
			ArrivalsAndDepartures []Arrival `json:"arrivalsAndDepartures"`
		} `json:"entry"`
	} `json:"data"`
}

type agenciesWithCoverageResponse struct {
	Code int    `json:"code"`
	Text string `json:"text"`
	Data struct {
		List []struct {
			AgencyID string `json:"agencyId"`
		} `json:"list"`
	} `json:"data"`
}

type stopResponse struct {
	Code int    `json:"code"`
	Text string `json:"text"`
	Data struct {
		Entry *struct {
			ID string `json:"id"`
		} `json:"entry"`
	} `json:"data"`
}

func GetArrivals(apiKey, stopID string) ([]Arrival, error) {
	arrivals, _, err := GetArrivalsForStop(apiKey, stopID)
	return arrivals, err
}

func GetArrivalsForStop(apiKey, stopRef string) ([]Arrival, string, error) {
	return getArrivals(http.DefaultClient, baseURL, apiKey, stopRef)
}

func GetArrivalsFromURL(client *http.Client, requestURL string) ([]Arrival, error) {
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("get arrivals: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unexpected HTTP status %d and failed to read body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var obaResp obaResponse
	if err := json.Unmarshal(body, &obaResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if obaResp.Code != http.StatusOK {
		return nil, fmt.Errorf("onebusaway error %d: %s", obaResp.Code, obaResp.Text)
	}

	return deduplicateArrivals(obaResp.Data.Entry.ArrivalsAndDepartures), nil
}

func getArrivals(client *http.Client, apiBaseURL, apiKey, stopRef string) ([]Arrival, string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	resolvedStopID, err := resolveStopID(client, apiBaseURL, apiKey, stopRef)
	if err != nil {
		return nil, "", err
	}

	arrivals, err := GetArrivalsFromURL(client, arrivalsURL(apiBaseURL, apiKey, resolvedStopID))
	if err != nil {
		return nil, "", err
	}

	return arrivals, resolvedStopID, nil
}

func resolveStopID(client *http.Client, apiBaseURL, apiKey, stopRef string) (string, error) {
	if !isBareStopCode(stopRef) {
		return stopRef, nil
	}

	agencyIDs, err := agenciesWithCoverage(client, apiBaseURL, apiKey)
	if err != nil {
		return "", fmt.Errorf("resolve stop code %s: %w", stopRef, err)
	}

	matches := make([]string, 0, 1)
	for _, agencyID := range agencyIDs {
		candidate := agencyID + "_" + stopRef
		exists, resolvedStopID, err := stopExists(client, apiBaseURL, apiKey, candidate)
		if err != nil {
			return "", fmt.Errorf("resolve stop code %s: %w", stopRef, err)
		}
		if exists {
			matches = append(matches, resolvedStopID)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("stop code %s was not found in Puget Sound OneBusAway; use a full stop ID if you already know it", stopRef)
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf("stop code %s matched multiple Puget Sound stop IDs: %s; use a full stop ID", stopRef, joinCommaSeparated(matches))
	}
}

func agenciesWithCoverage(client *http.Client, apiBaseURL, apiKey string) ([]string, error) {
	requestURL := fmt.Sprintf("%s/api/where/agencies-with-coverage.json?%s", apiBaseURL, url.Values{"key": []string{apiKey}}.Encode())

	resp, err := client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("get agencies with coverage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unexpected HTTP status %d and failed to read body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var coverageResp agenciesWithCoverageResponse
	if err := json.Unmarshal(body, &coverageResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if coverageResp.Code != http.StatusOK {
		return nil, fmt.Errorf("onebusaway error %d: %s", coverageResp.Code, coverageResp.Text)
	}

	seen := make(map[string]bool)
	agencyIDs := make([]string, 0, len(coverageResp.Data.List))
	for _, coverage := range coverageResp.Data.List {
		if coverage.AgencyID == "" || seen[coverage.AgencyID] {
			continue
		}
		seen[coverage.AgencyID] = true
		agencyIDs = append(agencyIDs, coverage.AgencyID)
	}

	sort.Strings(agencyIDs)
	if len(agencyIDs) == 0 {
		return nil, fmt.Errorf("no agencies with coverage returned")
	}

	return agencyIDs, nil
}

func stopExists(client *http.Client, apiBaseURL, apiKey, stopID string) (bool, string, error) {
	requestURL := fmt.Sprintf("%s/api/where/stop/%s.json?%s", apiBaseURL, url.PathEscape(stopID), url.Values{"key": []string{apiKey}}.Encode())

	resp, err := client.Get(requestURL)
	if err != nil {
		return false, "", fmt.Errorf("get stop %s: %w", stopID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, "", fmt.Errorf("check stop %s: unexpected HTTP status %d and failed to read body: %w", stopID, resp.StatusCode, err)
		}
		return false, "", fmt.Errorf("check stop %s: unexpected HTTP status %d: %s", stopID, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("check stop %s: read response body: %w", stopID, err)
	}

	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return false, "", nil
	}

	var stopResp stopResponse
	if err := json.Unmarshal(body, &stopResp); err != nil {
		return false, "", fmt.Errorf("check stop %s: unmarshal response: %w", stopID, err)
	}

	if stopResp.Code != http.StatusOK {
		return false, "", fmt.Errorf("check stop %s: onebusaway error %d: %s", stopID, stopResp.Code, stopResp.Text)
	}

	if stopResp.Data.Entry == nil || stopResp.Data.Entry.ID == "" {
		return false, "", nil
	}

	return true, stopResp.Data.Entry.ID, nil
}

func arrivalsURL(apiBaseURL, apiKey, stopID string) string {
	return fmt.Sprintf("%s/api/where/arrivals-and-departures-for-stop/%s.json?%s", apiBaseURL, url.PathEscape(stopID), url.Values{"key": []string{apiKey}}.Encode())
}

func isBareStopCode(stopRef string) bool {
	if stopRef == "" {
		return false
	}

	for _, r := range stopRef {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func joinCommaSeparated(values []string) string {
	if len(values) == 0 {
		return ""
	}

	result := values[0]
	for _, value := range values[1:] {
		result += ", " + value
	}
	return result
}

func deduplicateArrivals(arrivals []Arrival) []Arrival {
	seen := make(map[string]bool)
	result := make([]Arrival, 0, len(arrivals))
	for _, a := range arrivals {
		if a.TripID == "" || !seen[a.TripID] {
			if a.TripID != "" {
				seen[a.TripID] = true
			}
			result = append(result, a)
		}
	}
	return result
}
