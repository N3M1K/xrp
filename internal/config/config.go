package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	TLD            string `mapstructure:"tld"`
	PollInterval   int    `mapstructure:"poll_interval"`
	CaddyPort      int    `mapstructure:"caddy_port"`
	HTTPPort       int    `mapstructure:"http_port"`
	HTTPSPort      int    `mapstructure:"https_port"`
	LogLevel       string `mapstructure:"log_level"`
	KnownPortsPath string `mapstructure:"known_ports_path"`
}

// IsAdmin checks if the current process has admin/elevated privileges on Windows.
func IsAdmin() bool {
	// Try opening a file that requires admin rights
	f, err := os.Open(`\\.\PHYSICALDRIVE0`)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "xrp")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.toml")

	viper.SetConfigFile(configFile)
	viper.SetConfigType("toml")

	// Set defaults — ports depend on admin status
	viper.SetDefault("tld", ".test")
	viper.SetDefault("poll_interval", 5)
	viper.SetDefault("caddy_port", 2019)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("known_ports_path", "")

	// Auto-detect: admin gets standard ports, non-admin gets high ports
	if IsAdmin() {
		viper.SetDefault("http_port", 80)
		viper.SetDefault("https_port", 443)
	} else {
		viper.SetDefault("http_port", 8080)
		viper.SetDefault("https_port", 8443)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(*os.PathError); ok || os.IsNotExist(err) {
			// Config file not found; create it with defaults
			if writeErr := viper.SafeWriteConfigAs(configFile); writeErr != nil {
				return nil, fmt.Errorf("could not write default config: %w", writeErr)
			}
		} else {
			return nil, fmt.Errorf("could not read config: %w", err)
		}
	}

	var conf Config
	if err := viper.Unmarshal(&conf); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return &conf, nil
}
