package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/YusufHosny/hum/internal/chat"
	"github.com/YusufHosny/hum/internal/p2p"
)

func main() {
	username := flag.String("u", "", "Username for this peer (required)")
	channelName := flag.String("c", "test-room", "Channel name to join")
	signalingBase := flag.String("s", "ws://127.0.0.1:8787", "Base URL for the signaling server")

	flag.Parse()

	if *username == "" {
		fmt.Println("Error: Username is required.")
		flag.Usage()
		os.Exit(1)
	}

	// wss://worker.dev/<channelName>?usr=<username>
	rawURL := fmt.Sprintf("%s/%s?usr=%s", *signalingBase, *channelName, *username)
	signalingURL, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("Failed to parse signaling URL: %v", err)
	}

	log.Printf("Starting hum peer: %s", *username)
	log.Printf("Joining channel: %s", *channelName)
	log.Printf("Signaling server: %s", signalingURL.String())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("\nReceived signal: %v. Shutting down...", sig)
		cancel()
	}()

	manager, err := p2p.NewMeshManager(ctx, *signalingURL, *username, *channelName)
	if err != nil {
		log.Fatalf("Failed to initialize MeshManager: %v", err)
	}

	chatPipe := manager.GetChatPipe()

	// Handle incoming messages
	chatPipe.SetRecvHandler(func(env chat.ChatEnvelope) {
		if env.Type == "message" {
			// Print over current line to somewhat handle interleaved typing
			fmt.Printf("\r\033[K[%s]: %s\n> ", env.From, string(env.Content))
		}
	})

	log.Println("Peer is running. Type a message and press Enter to send.")

	// Start reading from stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ")
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text != "" {
				chatPipe.SendMessage(text)
			}
			fmt.Print("> ")
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading standard input: %v", err)
		}
	}()

	<-ctx.Done()

	err = manager.Close()
	if err != nil {
		log.Printf("Error during shutdown: %v", err)
	} else {
		log.Println("Shutdown complete.")
	}
}
