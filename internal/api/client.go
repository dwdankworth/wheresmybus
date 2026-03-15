package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const baseURL = "https://api.pugetsound.onebusaway.org"
const pugetSoundAgencyID = "1"

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

	body, err := readOKResponseBody(resp)
	if err != nil {
		return nil, err
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

	return fmt.Sprintf("%s_%s", pugetSoundAgencyID, stopRef), nil
}

func arrivalsURL(apiBaseURL, apiKey, stopID string) string {
	return fmt.Sprintf("%s/api/where/arrivals-and-departures-for-stop/%s.json?%s", apiBaseURL, url.PathEscape(stopID), url.Values{"key": []string{apiKey}}.Encode())
}

func readOKResponseBody(resp *http.Response) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unexpected HTTP status %d and failed to read body: %w", resp.StatusCode, err)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("rate limited by OneBusAway (HTTP 429): %s", string(body))
		}
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return body, nil
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
