package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mazanax/logtail-udp/version"
	"gopkg.in/robfig/cron.v2"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
		log.Fatalf("Err: LOGTAIL_TOKEN required\n")
	}

	listenHost := os.Getenv("LISTEN_HOST")
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}

	if net.ParseIP(listenHost) == nil {
		log.Fatalf("Err: LISTEN_HOST invalid\n")
	}

	listenPortStr := os.Getenv("LISTEN_PORT")
	if listenPortStr == "" {
		listenPortStr = "49152"
	}
	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		log.Fatalf("Err: LISTEN_PORT invalid (%s)\n", err)
	}
	if listenPort <= 0 || listenPort > 65535 {
		log.Fatalf("Err: LISTEN_PORT invalid\n")
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
		log.Fatalf("Err: LOGS_BATCH_SIZE invalid (%s)\n", err)
	}

	ctx, cancelFunction := context.WithCancel(context.Background())
	logsBuffer := make(chan LogEntry, logsBatchSize)

	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	fmt.Printf("Started cron job with interval = %s\n", cronInterval)
	wg.Add(1)
	go startCronJob(ctx, wg, logtailToken, logsBuffer, cronInterval)

	fmt.Printf("Listening %s:%d\n", listenHost, listenPort)
	go startListeningUDP(ctx, wg, listenHost, listenPort, logsBuffer)

	<-ctrlC
	cancelFunction()
	wg.Wait()
	sendBufferToLogtail(logtailToken, logsBuffer)
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
			fmt.Printf("Debug: stopped\n")
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
		log.Fatalf("Err: Cannot listen %s:%d (%s)\n", host, port, err)
	}
	defer conn.Close()

	buffer := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			fmt.Printf("Stop listening\n")
			return
		default:
			// go forward
		}

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Err: %s\n", err)
			return
		}

		received := string(buffer[0:n])
		fmt.Printf("Debug: received %s from %s\n", received, remoteAddr.String())
		logsBuffer <- LogEntry{remoteAddr.String(), received}
	}
}

func sendBufferToLogtail(logtailToken string, logsBuffer chan LogEntry) {
	payload := readAll(logsBuffer)
	fmt.Printf("Debug: have %d messages\n", len(payload))
	if len(payload) == 0 {
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
	fmt.Printf("Response: Status = %s\n", response.Status) // body always empty so we have to know only status
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
