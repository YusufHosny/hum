package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Username       string   `json:"username"`
	SignalingURL   string   `json:"signalingUrl"`
	STUNServers    []string `json:"stunServers"`
	InputVolume    float64  `json:"inputVolume"`
	OutputVolume   float64  `json:"outputVolume"`
	RecentChannels []string `json:"recentChannels"`
}

func GetConfigDir() (string, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, "hum")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return "", err
	}
	return configDir, nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() (*AppConfig, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config
			return &AppConfig{
				Username:       "",
				SignalingURL:   "wss://hum-signaling.worker.dev", // Replace with an actual default later
				STUNServers:    []string{"stun:stun.l.google.com:19302"},
				InputVolume:    1.0,
				OutputVolume:   1.0,
				RecentChannels: []string{},
			}, nil
		}
		return nil, err
	}

	var config AppConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Apply defaults for any missing fields
	if len(config.STUNServers) == 0 {
		config.STUNServers = []string{"stun:stun.l.google.com:19302"}
	}
	if config.SignalingURL == "" {
		config.SignalingURL = "wss://hum-signaling.worker.dev"
	}
	if config.InputVolume == 0 {
		config.InputVolume = 1.0
	}
	if config.OutputVolume == 0 {
		config.OutputVolume = 1.0
	}

	return &config, nil
}

func SaveConfig(config *AppConfig) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func AddRecentChannel(config *AppConfig, channelName string) {
	// Add to front if not exists, max 10 channels
	for i, c := range config.RecentChannels {
		if c == channelName {
			config.RecentChannels = append(config.RecentChannels[:i], config.RecentChannels[i+1:]...)
			break
		}
	}
	config.RecentChannels = append([]string{channelName}, config.RecentChannels...)
	if len(config.RecentChannels) > 10 {
		config.RecentChannels = config.RecentChannels[:10]
	}
}
