package integration

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// StabilityMetrics aggregates WebSocket test results
type StabilityMetrics struct {
	TotalConnAttempts    int64
	SuccessfulConns      int64
	TotalReconns         int64
	SuccessfulReconns    int64
	MessagesSent         int64
	MessagesReceived     int64
	ExpectedMessages     int64
	PacketLossRate       float64
	MinLatency           time.Duration
	MaxLatency           time.Duration
	AvgLatency           time.Duration
	Throughput           float64 // Msg/Sec
	ConnSuccessRate      float64
	ReconnSuccessRate    float64
}

// WSClient simulates a single WebSocket client session
type WSClient struct {
	id              int
	config          TestConfig
	token           string
	metrics         *StabilityMetrics
	latencyList     []time.Duration
	latencyMu       sync.Mutex
	t               *testing.T
}

func NewWSClient(id int, cfg TestConfig, token string, metrics *StabilityMetrics, t *testing.T) *WSClient {
	return &WSClient{
		id:      id,
		config:  cfg,
		token:   token,
		metrics: metrics,
		t:       t,
	}
}

func (c *WSClient) recordLatency(dur time.Duration) {
	c.latencyMu.Lock()
	defer c.latencyMu.Unlock()
	c.latencyList = append(c.latencyList, dur)
}

func (c *WSClient) runSession(wg *sync.WaitGroup, stopChan <-chan struct{}) {
	defer wg.Done()

	// Build ws connection URL
	wsURL := fmt.Sprintf("%s?token=%s", c.config.WSURL, url.QueryEscape(c.token))

	var conn *websocket.Conn
	var err error

	atomic.AddInt64(&c.metrics.TotalConnAttempts, 1)

	// Establish connection
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err = dialer.Dial(wsURL, nil)
	if err != nil {
		c.t.Logf("[Client %d] Initial Dial connection failed: %v", c.id, err)
		return
	}
	atomic.AddInt64(&c.metrics.SuccessfulConns, 1)
	defer conn.Close()

	// Maintain session loop
	startTime := time.Now()
	for {
		select {
		case <-stopChan:
			return
		default:
			// If session has exceeded duration, terminate naturally
			if time.Since(startTime) >= c.config.Duration {
				return
			}

			// Simulating Network Jitter: Add a random sleep delay before sending messages
			if c.config.JitterMax > 0 {
				sleepTime := time.Duration(rand.Int63n(int64(c.config.JitterMax)))
				time.Sleep(sleepTime)
			}

			// Send prompt request
			payload := map[string]string{
				"type":         "message",
				"dreamContent": fmt.Sprintf("在深渊中漫步 %d", rand.Intn(1000)),
			}

			msgBytes, _ := json.Marshal(payload)
			
			sendTime := time.Now()
			err = conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				c.t.Logf("[Client %d] Write failed, attempting reconnection: %v", c.id, err)
				conn = c.handleReconnection(wsURL)
				if conn == nil {
					return // Reconnection failed, terminate session
				}
				continue
			}
			atomic.AddInt64(&c.metrics.MessagesSent, 1)

			// Expected response size: 15 chunks (as defined in our Mock Stream Dream)
			expectedChunks := int64(15)
			atomic.AddInt64(&c.metrics.ExpectedMessages, expectedChunks)

			var receivedChunks int64 = 0
			firstChunkReceived := false
			streamFinished := false

			// Read stream chunks
			for !streamFinished {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					// Client disconnection occurred: record packets lost
					lostPackets := expectedChunks - receivedChunks
					if lostPackets < 0 {
						lostPackets = 0
					}
					atomic.AddInt64(&c.metrics.MessagesReceived, receivedChunks)
					c.t.Logf("[Client %d] Connection lost during read (%d packets lost): %v", c.id, lostPackets, err)

					// Trigger reconnection
					conn = c.handleReconnection(wsURL)
					if conn == nil {
						return // Reconnection failed, terminate session
					}
					break // break stream read loop and retry session actions
				}

				if !firstChunkReceived {
					c.recordLatency(time.Since(sendTime))
					firstChunkReceived = true
				}

				var chatMsg struct {
					Type    string `json:"type"`
					Content string `json:"content"`
					Error   string `json:"error"`
				}

				if err := json.Unmarshal(msg, &chatMsg); err == nil {
					if chatMsg.Type == "message" {
						receivedChunks++
					} else if chatMsg.Type == "done" || chatMsg.Type == "error" {
						atomic.AddInt64(&c.metrics.MessagesReceived, receivedChunks)
						streamFinished = true
					}
				}
			}

			// Simulating random disconnection/dirty connection close based on DisconnectProb
			if rand.Float64() < c.config.DisconnectProb {
				c.t.Logf("[Client %d] Simulating random dirty network disconnection...", c.id)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Simulated network drop"))
				conn.Close()

				// Immediate Reconnection
				conn = c.handleReconnection(wsURL)
				if conn == nil {
					return // failed to reconnect
				}
			}
		}
	}
}

// handleReconnection simulates the reconnection protocol
func (c *WSClient) handleReconnection(wsURL string) *websocket.Conn {
	atomic.AddInt64(&c.metrics.TotalReconns, 1)
	
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	
	// Max 3 reconnection attempts with exponential backoff
	backoff := 500 * time.Millisecond
	for i := 0; i < 3; i++ {
		time.Sleep(backoff)
		backoff *= 2

		conn, _, err := dialer.Dial(wsURL, nil)
		if err == nil {
			atomic.AddInt64(&c.metrics.SuccessfulReconns, 1)
			c.t.Logf("[Client %d] Reconnection success on attempt %d", c.id, i+1)
			return conn
		}
	}

	c.t.Logf("[Client %d] Reconnection failed after 3 attempts", c.id)
	return nil
}

// TestWebSocketStability performs a rigorous concurrent stress test on WebSocket
func TestWebSocketStability(t *testing.T) {
	cfg := LoadConfig()
	token, _ := GenerateTestToken(1, "stability_tester", cfg.JWTSecret)

	metrics := &StabilityMetrics{
		MinLatency: 9999 * time.Second,
	}

	t.Logf("Starting WebSocket Stability Test Suite...")
	t.Logf("Configurations: Concurrency=%d, Duration=%v, JitterMax=%v, Latency=%v, DisconnectProb=%.2f",
		cfg.Concurrency, cfg.Duration, cfg.JitterMax, cfg.Latency, cfg.DisconnectProb)

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	clients := make([]*WSClient, cfg.Concurrency)
	startTime := time.Now()

	// Spawn concurrent test runners
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		clients[i] = NewWSClient(i+1, cfg, token, metrics, t)
		go clients[i].runSession(&wg, stopChan)
	}

	// Wait for duration to complete
	time.Sleep(cfg.Duration)
	close(stopChan)

	// Wait for all goroutines to finish cleanups
	wg.Wait()
	totalTestTime := time.Since(startTime)

	// Collect and process all latency metrics
	var totalLatency time.Duration
	var latencyCount int64

	for _, client := range clients {
		client.latencyMu.Lock()
		for _, l := range client.latencyList {
			totalLatency += l
			latencyCount++
			if l < metrics.MinLatency {
				metrics.MinLatency = l
			}
			if l > metrics.MaxLatency {
				metrics.MaxLatency = l
			}
		}
		client.latencyMu.Unlock()
	}

	if latencyCount > 0 {
		metrics.AvgLatency = totalLatency / time.Duration(latencyCount)
	} else {
		metrics.MinLatency = 0
		metrics.MaxLatency = 0
	}

	// Calculate metrics rates
	if metrics.TotalConnAttempts > 0 {
		metrics.ConnSuccessRate = float64(metrics.SuccessfulConns) / float64(metrics.TotalConnAttempts) * 100.0
	}
	if metrics.TotalReconns > 0 {
		metrics.ReconnSuccessRate = float64(metrics.SuccessfulReconns) / float64(metrics.TotalReconns) * 100.0
	} else {
		metrics.ReconnSuccessRate = 100.0 // No reconns requested represents 100% success rate
	}
	if metrics.ExpectedMessages > 0 {
		received := metrics.MessagesReceived
		if received > metrics.ExpectedMessages {
			received = metrics.ExpectedMessages
		}
		metrics.PacketLossRate = float64(metrics.ExpectedMessages-received) / float64(metrics.ExpectedMessages) * 100.0
	}
	metrics.Throughput = float64(metrics.MessagesReceived) / totalTestTime.Seconds()

	// Render beautiful text report
	fmt.Printf("\n============================================================\n")
	fmt.Printf("              WEBSOCKET STABILITY TEST REPORT               \n")
	fmt.Printf("============================================================\n")
	fmt.Printf("Total Connection Attempts : %d\n", metrics.TotalConnAttempts)
	fmt.Printf("Successful Connections    : %d\n", metrics.SuccessfulConns)
	fmt.Printf("Connection Success Rate   : %.2f %%\n", metrics.ConnSuccessRate)
	fmt.Printf("------------------------------------------------------------\n")
	fmt.Printf("Total Reconnections       : %d\n", metrics.TotalReconns)
	fmt.Printf("Successful Reconnections  : %d\n", metrics.SuccessfulReconns)
	fmt.Printf("Reconnection Success Rate : %.2f %%\n", metrics.ReconnSuccessRate)
	fmt.Printf("------------------------------------------------------------\n")
	fmt.Printf("Messages Sent             : %d\n", metrics.MessagesSent)
	fmt.Printf("Messages Received         : %d\n", metrics.MessagesReceived)
	fmt.Printf("Expected Messages         : %d\n", metrics.ExpectedMessages)
	fmt.Printf("Packet Loss Rate          : %.2f %%\n", metrics.PacketLossRate)
	fmt.Printf("------------------------------------------------------------\n")
	fmt.Printf("Throughput                : %.2f msg/sec\n", metrics.Throughput)
	fmt.Printf("Min/Max/Avg Latency       : %v / %v / %v\n",
		metrics.MinLatency.Round(time.Millisecond),
		metrics.MaxLatency.Round(time.Millisecond),
		metrics.AvgLatency.Round(time.Millisecond))
	fmt.Printf("============================================================\n\n")

	// Rigorous Assertions
	if metrics.ConnSuccessRate < 95.0 {
		t.Errorf("FAIL: Connection Success Rate too low: %.2f%% (Expected > 95%%)", metrics.ConnSuccessRate)
	}
	if metrics.ReconnSuccessRate < 90.0 {
		t.Errorf("FAIL: Reconnection Success Rate too low: %.2f%% (Expected > 90%%)", metrics.ReconnSuccessRate)
	}
	if metrics.PacketLossRate > 5.0 {
		t.Errorf("FAIL: Packet Loss Rate too high: %.2f%% (Expected < 5%%)", metrics.PacketLossRate)
	}
}
