package data

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LastLogfile string `json:"last_logfile"`
	OpenOverlay bool   `json:"open_overlay"`
}

func (c *Config) Save() error {
	path, err := appDataPath("config.json")
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	err = os.WriteFile(path, bytes, 0644)
	return err
}

func GetConfig() (*Config, error) {
	config := &Config{}
	path, err := appDataPath("config.json")
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}
	err = json.Unmarshal(bytes, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func appDataPath(filename string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "eqdps")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return filepath.Join(dir, filename), nil
}
