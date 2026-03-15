package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	url := fmt.Sprintf("%s/api/where/arrivals-and-departures-for-stop/%s.json?key=%s", baseURL, stopID, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get arrivals: %w", err)
	}
	defer resp.Body.Close()

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

	return obaResp.Data.Entry.ArrivalsAndDepartures, nil
}
