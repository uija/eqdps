package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type EventType int8

const (
	EventTypeUndefined = -1
	EventTypeString    = iota
	EventTypeRegexp
	EventTypeSpell
	EventTypeTimer
)

type EventConfig struct {
	Type                EventType `json:"type"`
	Title               string    `json:"title"`
	Class               string    `json:"class,omitempty"`
	Spell               string    `json:"spell,omitempty"`
	Expression          string    `json:"expression"`
	ExpressionOthers    string    `json:"expression_others"`
	FullExpression      bool      `json:"full_expression"`
	Duration            int       `json:"duration,omitempty"`
	Notification        string    `json:"notification"`
	PersistNotification bool      `json:"persist_notification"`
	Sound               string    `json:"sound"`
	Active              bool      `json:"active"`
	RegExp              *regexp.Regexp
	RegExpOthers        *regexp.Regexp
}
type EQLDbConfig struct {
	UploadProfile             bool      `json:"upload_profile"`
	ContributeKillAndLootData bool      `json:"contribute"`
	AccessToken               string    `json:"access_token"`
	AuthorizationTime         time.Time `json:"authorization_time"`
}
type UIConfig struct {
	OverlayFontScale float32
}
type Config struct {
	LastLogfile string        `json:"last_logfile"`
	OpenOverlay bool          `json:"open_overlay"`
	Events      []EventConfig `json:"events"`
	Volume      float32       `json:"volume"`
	EQLDbConfig EQLDbConfig   `json:"eqldb"`
	UIConfig    UIConfig      `json:"ui"`
}

func (c *Config) Save() error {
	path, err := AppDataPath("config.json")
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
	config := &Config{
		Events: make([]EventConfig, 0),
		Volume: 0.5,
		UIConfig: UIConfig{
			OverlayFontScale: 1.0,
		},
	}
	path, err := AppDataPath("config.json")
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

func AppDataPath(filename string) (string, error) {
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
