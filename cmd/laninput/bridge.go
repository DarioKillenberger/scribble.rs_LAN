package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type keyboardInput struct {
	KeyboardID string `json:"keyboardId"`
	Key        string `json:"key"`
	Action     string `json:"action"`
}

type queuedKeyboardInput struct {
	input      keyboardInput
	capturedAt time.Time
}

const (
	inputQueueSize       = 4096
	inputPostTimeout     = 2 * time.Second
	inputMaxQueueAge     = 5 * time.Second
	inputRetryDelay      = 75 * time.Millisecond
	inputErrorLogEvery   = 2 * time.Second
	inputDroppedLogEvery = 2 * time.Second
)

var (
	lanHTTPClient = &http.Client{Timeout: inputPostTimeout}

	inputQueue        chan queuedKeyboardInput
	droppedInputCount atomic.Uint64
	lastInputPostLog  atomic.Int64
	lastInputDropLog  atomic.Int64
)

func runStdinBridge(endpoint, token, userSession string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var input keyboardInput
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			log.Printf("invalid input line: %v", err)
			continue
		}
		if input.Action == "" {
			input.Action = "keydown"
		}
		if err := postInput(endpoint, token, userSession, input); err != nil {
			log.Printf("post failed: %v", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func startInputPoster(endpoint, token, userSession string) {
	inputQueue = make(chan queuedKeyboardInput, inputQueueSize)
	go func() {
		for queued := range inputQueue {
			if time.Since(queued.capturedAt) > inputMaxQueueAge {
				logInputErrorThrottled("dropping stale keyboard input after local queue delay")
				continue
			}
			if err := postInput(endpoint, token, userSession, queued.input); err != nil {
				time.Sleep(inputRetryDelay)
				if retryErr := postInput(endpoint, token, userSession, queued.input); retryErr != nil {
					logInputErrorThrottled(fmt.Sprintf("post failed: %v", retryErr))
				}
			}
		}
	}()
}

func enqueueInput(input keyboardInput) {
	if inputQueue == nil {
		return
	}
	select {
	case inputQueue <- queuedKeyboardInput{input: input, capturedAt: time.Now()}:
	default:
		droppedInputCount.Add(1)
		logInputDropThrottled()
	}
}

func logInputErrorThrottled(message string) {
	now := time.Now().UnixNano()
	last := lastInputPostLog.Load()
	if time.Duration(now-last) < inputErrorLogEvery {
		return
	}
	if lastInputPostLog.CompareAndSwap(last, now) {
		log.Print(message)
	}
}

func logInputDropThrottled() {
	now := time.Now().UnixNano()
	last := lastInputDropLog.Load()
	if time.Duration(now-last) < inputDroppedLogEvery {
		return
	}
	if lastInputDropLog.CompareAndSwap(last, now) {
		log.Printf("keyboard input queue full; dropped %d events so far", droppedInputCount.Load())
	}
}

func postInput(endpoint, token, userSession string, input keyboardInput) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Lan-Control-Token", token)
	}
	if userSession != "" {
		request.Header.Set("Usersession", userSession)
	}

	response, err := lanHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		detail := strings.TrimSpace(string(responseBody))
		if detail == "" {
			return fmt.Errorf("server returned %s for %s", response.Status, endpoint)
		}
		return fmt.Errorf("server returned %s for %s: %s", response.Status, endpoint, detail)
	}
	return nil
}
