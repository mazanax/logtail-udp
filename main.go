package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mazanax/logtail-udp/version"
	"gopkg.in/robfig/cron.v2"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
)

type LogEntry struct {
	ip      string
	payload string
}

func main() {
	fmt.Println("[LogtailUDP]")
	fmt.Printf("Version: %s\n", version.Version())

	logtailToken := os.Getenv("LOGTAIL_TOKEN")
	if logtailToken == "" {
		fmt.Printf("Err: LOGTAIL_TOKEN required\n")
		os.Exit(2)
	}

	listenHost := os.Getenv("LISTEN_HOST")
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}

	if net.ParseIP(listenHost) == nil {
		fmt.Printf("Err: LISTEN_HOST invalid\n")
		os.Exit(2)
	}

	listenPortStr := os.Getenv("LISTEN_PORT")
	if listenPortStr == "" {
		listenPortStr = "49152"
	}
	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		fmt.Printf("Err: LISTEN_PORT invalid (%s)\n", err)
		os.Exit(2)
	}
	if listenPort <= 0 || listenPort > 65535 {
		fmt.Printf("Err: LISTEN_PORT invalid\n")
		os.Exit(2)
	}
	cronInterval := os.Getenv("LOGTAIL_SEND_INTERVAL")
	if cronInterval == "" {
		cronInterval = "10s"
	}
	logsBatchSizeStr := os.Getenv("LOGS_BATCH_SIZE")
	if logsBatchSizeStr == "" {
		logsBatchSizeStr = "64"
	}
	logsBatchSize, err := strconv.Atoi(logsBatchSizeStr)
	if err != nil {
		fmt.Printf("Err: LOGS_BATCH_SIZE invalid (%s)\n", err)
		os.Exit(2)
	}

	ctx, cancelFunction := context.WithCancel(context.Background())
	logsBuffer := make(chan LogEntry, logsBatchSize)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			fmt.Printf("Gracefully shuting down...")
			cancelFunction()
			sendBufferToLogtail(logtailToken, logsBuffer)
			os.Exit(1)
		}
	}()

	wg := &sync.WaitGroup{}

	fmt.Printf("Started cron job with interval = %s\n", cronInterval)
	wg.Add(1)
	go startCronJob(ctx, wg, logtailToken, logsBuffer, cronInterval)

	fmt.Printf("Listening %s:%d\n", listenHost, listenPort)
	wg.Add(1)
	startListeningUDP(ctx, wg, listenHost, listenPort, logsBuffer)

	wg.Wait()
	fmt.Println("Good bye!")
}

func startCronJob(ctx context.Context, wg *sync.WaitGroup, logtailToken string, logsBuffer chan LogEntry, cronInterval string) {
	c := cron.New()
	_, err := c.AddFunc("@every "+cronInterval, func() {
		sendBufferToLogtail(logtailToken, logsBuffer)
	})

	if err != nil {
		fmt.Printf("Err: %s\n", err)
		return
	}

	c.Start()
	for {
		select {
		case <-ctx.Done():
			c.Stop()
			wg.Done()
			fmt.Printf("Debug: c stopped\n")
			return
		default:
			// do nothing
		}
	}
}

func startListeningUDP(ctx context.Context, wg *sync.WaitGroup, host string, port int, logsBuffer chan LogEntry) {
	addr := &net.UDPAddr{IP: net.ParseIP(host), Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Err: Cannot listen %s:%d (%s)\n", host, port, err)
	}
	defer conn.Close()

	buffer := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Stop listening\n")
			wg.Done()
			return
		default:
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Printf("Err: %s\n", err)
				continue
			}

			received := string(buffer[0:n])
			fmt.Printf("Debug: received %s from %s\n", received, remoteAddr.String())
			logsBuffer <- LogEntry{remoteAddr.String(), received}
		}
	}
}

func sendBufferToLogtail(logtailToken string, logsBuffer chan LogEntry) {
	payload := readAll(logsBuffer)
	fmt.Printf("Debug: have %d messages\n", len(payload))
	if len(payload) < 3 {
		return
	}

	payloadMessages := make([]string, 0)
	for _, x := range payload {
		payloadMessages = append(payloadMessages, x.payload)
	}
	payloadString := fmt.Sprintf("[%s]", strings.Join(payloadMessages, ","))

	request, err := http.NewRequest("POST", "https://in.logtail.com", bytes.NewBufferString(payloadString))
	if err != nil {
		fmt.Printf("Err: cannot create http request (%s)\n", err)
		return
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", logtailToken))
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("Err: logtail error (%s)\n", err)
		return
	}
	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	fmt.Printf("Response: Status = %s, Body = %s\n", response.Status, string(body))
}

func readAll(logsBuffer chan LogEntry) []LogEntry {
	payload := make([]LogEntry, 0)

	for {
		select {
		case x, ok := <-logsBuffer:
			if ok {
				payload = append(payload, x)
			} else {
				return payload
			}
		default:
			return payload
		}
	}
}
