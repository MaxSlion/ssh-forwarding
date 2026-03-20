package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// AppSettings holds all user-configurable settings
type AppSettings struct {
	// Connection
	AgentPath        string `json:"agentPath"`        // Remote path to server-agent binary
	ConnectionTimeout int    `json:"connectionTimeout"` // SSH connection timeout in seconds
	LocalBindAddress string `json:"localBindAddress"`  // Default local bind address for forwarding
	AutoReconnect    bool   `json:"autoReconnect"`     // Auto-reconnect on disconnect

	// Appearance
	Theme    string `json:"theme"`    // "light" or "dark"
	Language string `json:"language"` // "zh" or "en"
}

func defaultSettings() AppSettings {
	return AppSettings{
		AgentPath:        "/server-agent",
		ConnectionTimeout: 10,
		LocalBindAddress: "127.0.0.1",
		AutoReconnect:    true,
		Theme:            "light",
		Language:         "zh",
	}
}

func settingsFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	dir := filepath.Join(configDir, "ssh-forwarder")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "settings.json")
}

// LoadSettings reads settings from disk, returns defaults if not found
func (a *App) LoadSettings() AppSettings {
	path := settingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		defaults := defaultSettings()
		if os.IsNotExist(err) {
			if saveErr := a.writeSettings(path, defaults); saveErr != nil {
				log.Printf("Settings file not found; using defaults (failed to create %s): %v", path, saveErr)
			} else {
				log.Printf("Settings file not found; created defaults at %s", path)
			}
			return defaults
		}

		log.Printf("Failed to read settings file, using defaults: %v", err)
		return defaults
	}

	settings := defaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("Invalid settings file, using defaults: %v", err)
		return defaultSettings()
	}
	return settings
}

// SaveSettings persists settings to disk
func (a *App) SaveSettings(settings AppSettings) bool {
	path := settingsFilePath()
	if err := a.writeSettings(path, settings); err != nil {
		log.Printf("Failed to save settings: %v", err)
		return false
	}
	log.Printf("Settings saved to %s", path)
	return true
}

func (a *App) writeSettings(path string, settings AppSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetSettingsPath returns the path to the settings file (for debug)
func (a *App) GetSettingsPath() string {
	return settingsFilePath()
}
