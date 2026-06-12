package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YusufHosny/hum/internal/client"
	"github.com/YusufHosny/hum/internal/config"
	"github.com/YusufHosny/hum/internal/logger"
	"github.com/YusufHosny/hum/internal/ui"
)

func main() {
	usernameFlag := flag.String("u", "", "Username for this peer (overrides config)")
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

	if *usernameFlag != "" {
		appConfig.Username = *usernameFlag
	}

	if *signalingBase != "" {
		appConfig.SignalingURL = *signalingBase
	}

	// Initialize the background Hum client
	humClient := client.NewClient(appConfig, appLogger)

	// Create and start the UI
	p := tea.NewProgram(
		ui.InitialModel(appConfig, appLogger, humClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	appLogger.Println("Starting UI...")
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	// Cleanup
	humClient.Disconnect()
	appLogger.Println("Shutdown complete.")
}
