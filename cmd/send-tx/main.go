package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
)

type NodeClient struct {
	Address string
	Client  *http.HTTP
}

type TxInfo struct {
	SendTime    int64          `json:"send_timestamp"`   // Unix milliseconds
	CommitTime  int64          `json:"commit_timestamp"` // Unix milliseconds
	Latency     int64          `json:"latency"`
	BlockHeight int64          `json:"block_height"`
	TxHash      bytes.HexBytes `json:"tx_hash"`
}

type Summarize struct {
	TotalSent    int     `json:"total_sent"`
	TotalSucceed int     `json:"total_succeed"`
	TPS          float64 `json:"tps"`
	TotlaLatency int64   `json:"total_latency"`
}

type CommitLog struct {
	Level   string    `json:"level"`
	Module  string    `json:"module"`
	Height  int64     `json:"height"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

type CommitTimeCache struct {
	cache map[int64]int64
	mutex sync.RWMutex
}

func NewCommitTimeCache() *CommitTimeCache {
	return &CommitTimeCache{
		cache: make(map[int64]int64),
	}
}

func (c *CommitTimeCache) Get(height int64) (int64, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	time, ok := c.cache[height]
	return time, ok
}

func (c *CommitTimeCache) Set(height int64, time int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[height] = time
}

var (
	encodedTxDir = flag.String("dir", "/node/txns/encoded", "encoded txs 디렉토리 경로")
	inputTPS     = flag.Int("tps", 100, "초당 전송할 트랜잭션 수")
	runTime      = flag.Int("time", 60, "실행 시간(초)")
	nodeCount    = flag.Int("nodes", 1, "노드 갯수")
	nodeLogFile  = flag.String("log", "/node/node1/node1.log", "노드 로그 파일 경로")

	nodeAddresses = []string{
		"tcp://localhost:26657",
		"tcp://localhost:26757",
		"tcp://localhost:26857",
		"tcp://localhost:26957",
	}
)

func FindCommitTimeInLog(logFile string, height int64) (int64, error) {
	wd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("failed to get working directory: %v\n", err)
	}
	file, err := os.Open(wd + logFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var log CommitLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue
		}

		if log.Height == height &&
			log.Module == "state" &&
			log.Message == "committed state" {
			return log.Time.UnixMilli(), nil
		}
	}

	return 0, fmt.Errorf("commit time not found for height %d", height)
}

func groupTxsByHeight(ctx context.Context, txInfos []TxInfo, client *http.HTTP, maxRetries int) map[int64][]*TxInfo {
	heightMap := make(map[int64][]*TxInfo)
	foundTxs := make(map[string]bool)

	attempt := 0
	for attempt < maxRetries {
		fmt.Printf("\nAttempt %d/%d to find transactions in blocks\n", attempt+1, maxRetries)

		// 아직 발견되지 않은 트랜잭션들에 대해서만 검색
		for i := range txInfos {
			if foundTxs[txInfos[i].TxHash.String()] {
				continue
			}

			txResponse, err := client.Tx(ctx, txInfos[i].TxHash, true)
			if err != nil {
				// fmt.Printf("Failed to fetch tx %s: %v\n", txInfos[i].TxHash.String(), err)
				continue
			}

			height := txResponse.Height
			txInfos[i].BlockHeight = height
			heightMap[height] = append(heightMap[height], &txInfos[i])
			foundTxs[txInfos[i].TxHash.String()] = true
		}

		// 모든 트랜잭션이 발견되었는지 확인
		if len(foundTxs) == len(txInfos) {
			fmt.Printf("All transactions found in blocks\n")
			break
		}

		fmt.Printf("Found %d/%d transactions. Waiting before retry...\n", len(foundTxs), len(txInfos))
		attempt++
		time.Sleep(5 * time.Second)
	}

	return heightMap
}

func FetchBlockInfoBatch(ctx context.Context, txInfos []TxInfo, client *http.HTTP, maxRetries int) []TxInfo {
	// 모든 트랜잭션이 발견될 때까지 재시도하면서 블록 높이별로 그룹화
	heightMap := groupTxsByHeight(ctx, txInfos, client, maxRetries)
	commitTimeCache := NewCommitTimeCache()

	var succeedInfos []TxInfo
	failed := make(map[int64]bool)

	// 각 블록 높이별로 commit 시간 찾기
	for height, txs := range heightMap {
		commitTime, err := FindCommitTimeInLog(*nodeLogFile, height)
		if err != nil {
			fmt.Printf("Failed to find commit time for height %d: %v\n", height, err)
			failed[height] = true
			continue
		}

		// commit 시간 캐시에 저장
		commitTimeCache.Set(height, commitTime)

		// 같은 블록의 모든 트랜잭션에 commit 시간 적용
		for _, tx := range txs {
			tx.CommitTime = commitTime
			tx.Latency = commitTime - tx.SendTime
			succeedInfos = append(succeedInfos, *tx)
		}
	}

	fmt.Printf("\nFinal results:\n")
	fmt.Printf("Successfully fetched commit time: %d\n", len(succeedInfos))
	fmt.Printf("Failed to fetch commit time for %d blocks\n", len(failed))

	if len(succeedInfos) == 0 {
		fmt.Printf("Warning: No transactions were successfully processed\n")
	}

	return succeedInfos
}

func ReadEncodedTxs(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find files: %v", err)
	}

	var txs []string
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file (%s): %v", file, err)
		}
		txs = append(txs, string(content))
	}
	return txs, nil
}

func SendTransaction(
	ctx context.Context,
	txIdx int,
	encodedTx string,
	nodeClients []*NodeClient,
	wg *sync.WaitGroup,
	txInfos chan<- TxInfo,
	fileMutex *sync.Mutex,
	logFile *os.File,
) {
	defer wg.Done()

	nodeIdx := txIdx % len(nodeClients)
	selectedNode := nodeClients[nodeIdx]

	txBytes, err := base64.StdEncoding.DecodeString(encodedTx)
	if err != nil {
		fmt.Printf("[TxSequence %d] Failed to decode tx: %v\n", txIdx, err)
		return
	}

	timestamp := time.Now().UTC().UnixMilli()

	res, err := selectedNode.Client.BroadcastTxSync(context.Background(), txBytes)
	if err != nil {
		fmt.Printf("[TxSequence %d, Node %d] Failed to broadcast tx: %v\n", txIdx, nodeIdx, err)
		return
	}
	txInfo := TxInfo{
		TxHash:   res.Hash,
		SendTime: timestamp,
	}
	txInfos <- txInfo

	fileMutex.Lock()
	fmt.Fprintf(logFile, "txIdx: %d txHash: %s time: %d node: %s\n",
		txIdx, res.Hash.String(), timestamp, selectedNode.Address)
	fileMutex.Unlock()
}

func main() {
	flag.Parse()

	ctx := context.Background()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
		return
	}

	txs, err := ReadEncodedTxs(wd + *encodedTxDir)
	if err != nil {
		fmt.Printf("Failed to read transaction data: %v\n", err)
		return
	}

	numTxs := len(txs)
	fmt.Printf("Total transactions: %d\n", numTxs)
	fmt.Printf("TPS: %d, Nodes: %d Runtime: %d seconds\n", *inputTPS, *nodeCount, *runTime)

	nodeClients := make([]*NodeClient, *nodeCount)
	for i := 0; i < *nodeCount; i++ {
		addr := nodeAddresses[i]
		client, err := client.NewClientFromNode(addr)
		if err != nil {
			fmt.Printf("Failed to create client for node %d: %v\n", i+1, err)
			return
		}
		nodeClients[i] = &NodeClient{addr, client}
	}

	logFile, err := os.Create("tx_log.txt")
	if err != nil {
		fmt.Printf("Failed to create log file: %v\n", err)
		return
	}
	defer logFile.Close()

	txInfos := make(chan TxInfo, numTxs)
	defer close(txInfos)

	var fileMutex sync.Mutex
	var wg sync.WaitGroup
	sentTxs := 0

	for i := 0; i < *runTime && sentTxs < numTxs; i++ {
		startTime := time.Now()

		remainingTxs := numTxs - sentTxs
		txsToSend := *inputTPS
		if remainingTxs < *inputTPS {
			txsToSend = remainingTxs
		}

		for j := 0; j < txsToSend; j++ {
			wg.Add(1)
			go SendTransaction(ctx, sentTxs+j, txs[sentTxs+j], nodeClients, &wg, txInfos, &fileMutex, logFile)
		}

		wg.Wait()
		sentTxs += txsToSend

		elapsedTime := time.Since(startTime).Milliseconds()
		if elapsedTime < 1000 {
			time.Sleep(time.Duration(1000-elapsedTime) * time.Millisecond)
		}

		if sentTxs >= numTxs {
			break
		}
	}

	fmt.Printf("All transactions sent (total: %d)\n", sentTxs)
	fmt.Printf("Fetching block information...\n")

	var allTxInfos []TxInfo
	for i := 0; i < sentTxs; i++ {
		txInfo := <-txInfos
		allTxInfos = append(allTxInfos, txInfo)
	}
	maxRetries := int(math.Ceil(float64(numTxs) / 50 * 1.5))
	succeedInfos := FetchBlockInfoBatch(ctx, allTxInfos, nodeClients[0].Client, maxRetries)

	sort.Slice(succeedInfos, func(i, j int) bool {
		return succeedInfos[i].SendTime < succeedInfos[j].SendTime
	})
	firstSendTime := succeedInfos[0].SendTime

	sort.Slice(succeedInfos, func(i, j int) bool {
		return succeedInfos[i].CommitTime < succeedInfos[j].CommitTime
	})
	lastCommitTime := succeedInfos[len(succeedInfos)-1].CommitTime

	totalSucceed := len(succeedInfos)
	totalLatency := lastCommitTime - firstSendTime
	tps := float64(totalSucceed) / (float64(totalLatency) / 1000)

	file, err := os.OpenFile("tx_results.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	summarizeData, err := json.Marshal(Summarize{TotalSent: numTxs, TotalSucceed: totalSucceed, TPS: tps, TotlaLatency: totalLatency})
	if err != nil {
		fmt.Printf("Failed to marshal summarize: %v\n", err)
	}
	file.Write(summarizeData)
	file.Write([]byte("\n"))

	for _, txInfo := range succeedInfos {
		jsonData, err := json.Marshal(txInfo)
		if err != nil {
			fmt.Printf("Failed to marshal tx info: %v\n", err)
			continue
		}

		_, err = file.Write(jsonData)
		if err != nil {
			fmt.Printf("Failed to write to file: %v\n", err)
			continue
		}

		file.Write([]byte("\n"))
	}

	fmt.Printf("Transaction results saved to tx_results.json\n")
	fmt.Printf("Summary:\n")
	fmt.Printf("Total sent: %d\n", numTxs)
	fmt.Printf("Total succeeded: %d\n", totalSucceed)
	fmt.Printf("Overall TPS: %.2f\n", tps)
	fmt.Printf("Total latency: %d ms\n", totalLatency)
}
