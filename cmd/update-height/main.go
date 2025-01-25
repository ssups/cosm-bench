package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

const (
	HOST      = "127.0.0.1"
	REST_PORT = "1317"
)

type LogEntry struct {
	TxIdx     int    `json:"txIdx"`
	Timestamp int64  `json:"timestamp"`
	TxHash    string `json:"txHash"`
	Height    int    `json:"height,omitempty"`
}

func queryHeight(txHash string, host string, port string) (int, error) {
	url := fmt.Sprintf("http://%s:%s/cosmos/tx/v1beta1/txs/%s", host, port, txHash)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to query height for txHash %s: %v", txHash, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("non-200 response: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	var queryResp struct {
		TxResponse struct {
			Height string `json:"height"`
		} `json:"tx_response"`
	}
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return 0, fmt.Errorf("failed to parse response JSON: %v", err)
	}

	height, err := strconv.Atoi(queryResp.TxResponse.Height)
	if err != nil {
		return 0, fmt.Errorf("failed to convert height to int: %v", err)
	}

	return height, nil
}

func main() {
	logFileName := "results/tx_log.json"

	data, err := ioutil.ReadFile(logFileName)
	if err != nil {
		fmt.Printf("Failed to read log file: %v\n", err)
		return
	}

	var logEntries []LogEntry
	if err := json.Unmarshal(data, &logEntries); err != nil {
		fmt.Printf("Failed to parse log file: %v\n", err)
		return
	}

	fmt.Println("[INFO] Updating transaction heights...")
	for i, log := range logEntries {
		height, err := queryHeight(log.TxHash, HOST, REST_PORT)
		if err != nil {
			fmt.Printf("[TxIdx %d] Failed to query height for txHash %s: %v\n", log.TxIdx, log.TxHash, err)
			continue
		}
		logEntries[i].Height = height
		// fmt.Printf("[TxIdx %d] Updated height: %d\n", log.TxIdx, height)
		if i%100 == 0 {
			fmt.Printf("Iteration: %d, [TxIdx %d] Updated height: %d\n", i, log.TxIdx, height)
		}
	}

	file, err := os.Create(logFileName)
	if err != nil {
		fmt.Printf("Failed to create updated log file: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(logEntries); err != nil {
		fmt.Printf("Failed to write updated log file: %v\n", err)
		return
	}

	fmt.Println("[INFO] Log file updated with heights.")
}
