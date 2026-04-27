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
	"path/filepath"
	"strings"
	"syscall"

	"github.com/YusufHosny/hum/internal/audio"
	"github.com/YusufHosny/hum/internal/chat"
	"github.com/YusufHosny/hum/internal/config"
	"github.com/YusufHosny/hum/internal/crypto"
	"github.com/YusufHosny/hum/internal/logger"
	"github.com/YusufHosny/hum/internal/p2p"
)

func main() {
	usernameFlag := flag.String("u", "", "Username for this peer (overrides config)")
	channelName := flag.String("c", "test-room", "Channel name to join")
	signalingBase := flag.String("s", "", "Base URL for the signaling server (overrides config)")

	flag.Parse()

	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		log.Fatalf("Failed to get config directory: %v", err)
	}

	appLogger := logger.New(filepath.Join(configDir, "hum.log"))

	username := appConfig.Username
	if *usernameFlag != "" {
		username = *usernameFlag
	}

	if username == "" {
		fmt.Println("Error: Username is required. Set it in config or pass -u flag.")
		flag.Usage()
		os.Exit(1)
	}
	appConfig.Username = username

	sigBase := appConfig.SignalingURL
	if *signalingBase != "" {
		sigBase = *signalingBase
	}
	appConfig.SignalingURL = sigBase

	config.AddRecentChannel(appConfig, *channelName)
	err = config.SaveConfig(appConfig)
	if err != nil {
		appLogger.Printf("Failed to save config: %v", err)
	}

	// wss://worker.dev/<channelName>?usr=<username>
	rawURL := fmt.Sprintf("%s/%s?usr=%s", appConfig.SignalingURL, *channelName, username)
	signalingURL, err := url.Parse(rawURL)
	if err != nil {
		appLogger.Fatalf("Failed to parse signaling URL: %v", err)
	}

	appLogger.Printf("Starting hum peer: %s", username)
	appLogger.Printf("Joining channel: %s", *channelName)
	appLogger.Printf("Signaling server: %s", signalingURL.String())

	fmt.Printf("Logging to: %s\n", filepath.Join(configDir, "hum.log"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		appLogger.Printf("\nReceived signal: %v. Shutting down...", sig)
		cancel()
	}()

	cryptor, err := crypto.NewCryptor(*channelName, "dummy-passkey")
	if err != nil {
		appLogger.Fatalf("Failed to initialize cryptor: %v", err)
	}

	chatManager := chat.NewChatManager(ctx, username, cryptor)

	audioConfig := audio.NewDefaultAudioConfig()
	audioConfig.InputVolume = appConfig.InputVolume
	audioConfig.OutputVolume = appConfig.OutputVolume

	audioManager, err := audio.NewAudioManager(ctx, audioConfig, cryptor)
	if err != nil {
		appLogger.Fatalf("Failed to initialize audio manager: %v", err)
	}

	err = audioManager.Start()
	if err != nil {
		appLogger.Fatalf("Failed to start audio manager: %v", err)
	}
	defer audioManager.Stop()

	meshConfig := p2p.MeshConfig{
		SignalingServerURL: *signalingURL,
		STUNServers:        appConfig.STUNServers,
		Username:           username,
		ChannelName:        *channelName,
		Logger:             appLogger,
	}

	manager, err := p2p.NewMeshManager(ctx, meshConfig, chatManager, audioManager)
	if err != nil {
		appLogger.Fatalf("Failed to initialize MeshManager: %v", err)
	}

	subChan := chatManager.Subscribe()

	// Handle incoming messages
	go func() {
		for env := range subChan {
			if env.Type == "message" {
				// Print over current line to somewhat handle interleaved typing
				fmt.Printf("\r\033[K[%s]: %s\n> ", env.From, string(env.Content))
			}
		}
	}()

	fmt.Println("Peer is running. Type a message and press Enter to send.")

	// Start reading from stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ")
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text != "" {
				chatManager.SendMessage(text)
			}
			fmt.Print("> ")
		}
		if err := scanner.Err(); err != nil {
			appLogger.Printf("Error reading standard input: %v", err)
		}
	}()

	<-ctx.Done()

	err = manager.Close()
	if err != nil {
		appLogger.Printf("Error during shutdown: %v", err)
	} else {
		appLogger.Println("Shutdown complete.")
	}
}
