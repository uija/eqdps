package data

import (
	"encoding/json"
	"log"
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
	ContributeKillAndLootData bool      `json:"contribute"`
	UploadSkyData             bool      `json:"upload_sky"`
	AccessToken               string    `json:"access_token"`
	AuthorizationTime         time.Time `json:"authorization_time"`
}
type UIConfig struct {
	MainWindowWidth  int     `json:"main_window_width"`
	MainWindowHeight int     `json:"main_window_height"`
	OverlayX         int     `json:"overlay_x"`
	OverlayY         int     `json:"overlay_y"`
	OverlayFontScale float32 `json:"overlay_font_scale"`
	OverlayOpacity   float32 `json:"overlay_opacity"`
}
type SkyConfig struct {
	ParseInventoryData bool `json:"parse_inventory_data"`
}
type Config struct {
	LastLogfile     string        `json:"last_logfile"`
	RecentLogFiles  []string      `json:"recent_logfiles"`
	OpenOverlay     bool          `json:"open_overlay"`
	Events          []EventConfig `json:"events"`
	Volume          float32       `json:"volume"`
	SpellIconSet    string        `json:"spell_icon_set"`
	EQLDbConfig     EQLDbConfig   `json:"eqldb"`
	SkyConfig       SkyConfig     `json:"sky"`
	UIConfig        UIConfig      `json:"ui"`
	CheckForUpdates bool          `json:"check_for_updates"`
	LastSeenVersion string        `json:"last_seen_version"`
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
			OverlayOpacity:   1.0,
		},
		EQLDbConfig: EQLDbConfig{
			UploadSkyData: true,
		},
		RecentLogFiles: make([]string, 0),
	}
	path, err := AppDataPath("config.json")
	log.Printf("Logpath: %s", path)
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
	if config.RecentLogFiles == nil {
		config.RecentLogFiles = make([]string, 0)
	}
	if config.UIConfig.OverlayOpacity < 0.2 {
		config.UIConfig.OverlayOpacity = 1.0
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
