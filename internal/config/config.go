package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	TLD            string            `mapstructure:"tld"`
	ProjectTLDs    map[string]string `mapstructure:"project_tlds"`
	PollInterval   int               `mapstructure:"poll_interval"`
	CaddyPort      int               `mapstructure:"caddy_port"`
	HTTPPort       int               `mapstructure:"http_port"`
	HTTPSPort      int               `mapstructure:"https_port"`
	LogLevel       string            `mapstructure:"log_level"`
	KnownPortsPath string            `mapstructure:"known_ports_path"`
}

func SetProjectTLD(projectName, tld string) error {
	projectTLDs := viper.GetStringMapString("project_tlds")
	if projectTLDs == nil {
		projectTLDs = make(map[string]string)
	}
	if tld == "" {
		delete(projectTLDs, projectName)
	} else {
		projectTLDs[projectName] = tld
	}
	viper.Set("project_tlds", projectTLDs)
	return viper.WriteConfig()
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
	viper.SetDefault("project_tlds", map[string]string{})
	viper.SetDefault("poll_interval", 5)
	viper.SetDefault("caddy_port", 2019)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("known_ports_path", "")

	// Force strict privileged ports globally (Admin escalation politely requested on failure)
	viper.SetDefault("http_port", 80)
	viper.SetDefault("https_port", 443)

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
