package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"
)

type Service struct {
	Source    string `json:"source"`
	Query     string `json:"query"`
	Aggregate string `json:"aggregate"`
	Threshold int    `json:"threshold"`
	Offset    []int  `json:"offset"`
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

var (
	prometheusHost string
	prometheusPort string
)

func main() {

	setConfigs()

	// Read the JSON file
	fileData, err := ioutil.ReadFile("config.json")
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	// Parse JSON data into a map of Service objects
	var services map[string]Service
	if err := json.Unmarshal(fileData, &services); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	for {
		// Iterate over the services and query Prometheus
		for serviceName, service := range services {
			go func(service Service, serviceName string) {
				if service.Source == "prometheus" {
					query := service.Aggregate + "(" + service.Query + ")"
					fmt.Printf("Querying Prometheus for service %s with query %s", serviceName, query)
					result, err := queryPrometheus(query)
					if err != nil {
						fmt.Println(err)
					} else {
						fmt.Printf("Value from Prometheus: %v\n", result)
						for _, offset := range service.Offset {
							queryWithOffset := service.Aggregate + "(" + service.Query + "%20offset%20" + strconv.Itoa(offset) + "d)"
							offsetResult, err := queryPrometheus(queryWithOffset)
							if err == nil {
								variance, ok := checkIfBreakingThreshold(offsetResult, result, service.Threshold)
								if ok {
									fmt.Printf("Difference %f", variance)
									monitorForSetTime(offsetResult, query, serviceName, service.Threshold)
								}
							}

						}
					}
				}
			}(service, serviceName)
		}
		time.Sleep(2 * time.Minute)
	}
}

func setConfigs() {
	prometheusHost = "https://prometheus.example.com"
	prometheusPort = "9090"
}

func queryPrometheus(query string) (float64, error) {
	prometheusURL := prometheusHost + ":" + prometheusPort + "/api/v1/query?query=" + query

	resp, err := http.Get(prometheusURL)
	if err != nil {
		fmt.Println("Error querying Prometheus:", err)
		return -1, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return -1, err
	}

	var response PrometheusResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Println("Error parsing Prometheus response:", err)
		return -1, err
	}

	if response.Status == "success" && len(response.Data.Result) > 0 {
		currentValue := response.Data.Result[0].Value[1]
		fmt.Printf("Value from Prometheus: %v\n", currentValue)
		v1, err := strconv.ParseFloat(currentValue.(string), 64)
		if err != nil {
			return -1, err
		}
		return v1, nil
	}
	return -1, nil
}

func monitorForSetTime(baseValue float64, query, serviceName string, threshold int) {
	fmt.Println("Sleeping for 5 minutes before recheck for service %s. Hold tight!", serviceName)
	time.Sleep(5 * time.Minute)
	result, err := queryPrometheus(query)
	if err == nil {
		checkIfBreakingThreshold(baseValue, result, threshold)
	}
}

func checkIfBreakingThreshold(v1, v2 float64, threshold int) (float64, bool) {
	percentDifference := ((v1 - v2) / v1) * 100
	absPercentDifference := math.Abs(percentDifference)
	if absPercentDifference >= float64(threshold) {
		return percentDifference, true
	}
	return percentDifference, false
}
