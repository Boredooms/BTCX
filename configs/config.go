// Package configs loads and manages BCTX local configuration (TOML) and the
// runtime directory layout under ~/.bctx. Configuration is always local; the
// loader never performs network access.
package configs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// NetworkMode controls whether acquisition may use the network.
type NetworkMode string

const (
	// ModeOnline permits acquisition adapters to use the network.
	ModeOnline NetworkMode = "online"
	// ModeOffline forbids acquisition for this run; analysis still works.
	ModeOffline NetworkMode = "offline"
	// ModeAirgap hard-disables all acquisition at the process level.
	ModeAirgap NetworkMode = "airgap"
)

// Config is the root configuration object.
type Config struct {
	App         AppConfig         `toml:"app"`
	Models      ModelsConfig      `toml:"models"`
	Monitor     MonitorConfig     `toml:"monitor"`
	Monitoring  MonitoringConfig  `toml:"monitoring"`
	Reports     ReportsConfig     `toml:"reports"`
	Network     NetworkConfig     `toml:"network"`
	Acquisition AcquisitionConfig `toml:"acquisition"`
	LLM         LLMConfig         `toml:"llm"`

	// path is the file this config was loaded from (not serialized).
	path string
}

// AppConfig holds core application settings.
type AppConfig struct {
	DataDir     string `toml:"data_dir"`
	DefaultCase string `toml:"default_case"`
	LogLevel    string `toml:"log_level"`
}

// ModelsConfig points at the local model directory.
type ModelsConfig struct {
	Directory string `toml:"directory"`
}

// MonitorConfig holds monitoring defaults.
type MonitorConfig struct {
	DefaultInterval string `toml:"default_interval"`
}

// ReportsConfig holds reporting defaults.
type ReportsConfig struct {
	DefaultFormat string `toml:"default_format"`
}

// NetworkConfig controls the acquisition boundary.
type NetworkConfig struct {
	// AcquisitionEnabled gates whether acquisition adapters may run at all.
	AcquisitionEnabled bool        `toml:"acquisition_enabled"`
	Mode               NetworkMode `toml:"mode"`
	// ExplorerEndpoint is the optional blockchain data provider base URL.
	ExplorerEndpoint string `toml:"explorer_endpoint"`
}

// MonitoringConfig controls the Phase 6 live monitor. Reuses [acquisition] for
// retry/rate-limit; only adds monitor-specific settings.
type MonitoringConfig struct {
	PollInterval         string `toml:"poll_interval"`
	MaxEventQueue        int    `toml:"max_event_queue"`
	AnalysisDebounce     string `toml:"analysis_debounce"`
	ReconnectBackoff     string `toml:"reconnect_backoff"`
	MaxReconnectAttempts int    `toml:"max_reconnect_attempts"`
	AlertRiskDelta       int    `toml:"alert_risk_delta"`
	AlertNewSignal       bool   `toml:"alert_new_signal"`
	AlertNewPattern      bool   `toml:"alert_new_pattern"`
	SessionRetention     int    `toml:"session_retention"`
}

// AcquisitionConfig controls the Phase 5 provider sync engine. All network use
// is confined to the acquisition layer; these settings never affect offline
// analysis.
type AcquisitionConfig struct {
	Provider          string  `toml:"provider"`            // provider name (e.g. "fake", "explorer")
	MaxWorkers        int     `toml:"max_workers"`         // bounded concurrent tx fetchers
	PageSize          int     `toml:"page_size"`           // wallet-history page size
	BatchSize         int     `toml:"batch_size"`          // canonical persist batch size
	RequestTimeout    string  `toml:"request_timeout"`     // per-request timeout (duration)
	MaxRetries        int     `toml:"max_retries"`         // retry attempts for retryable errors
	InitialBackoff    string  `toml:"initial_backoff"`     // backoff base (duration)
	MaxBackoff        string  `toml:"max_backoff"`         // backoff cap (duration)
	RequestsPerSecond float64 `toml:"requests_per_second"` // rate limit (0 = unlimited)
}

// LLMConfig holds optional local LLM settings (Ollama). The LLM is never the
// source of risk scores — only summaries of already-computed evidence.
type LLMConfig struct {
	Enabled  bool   `toml:"enabled"`
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
}

// Default returns a configuration with sane offline-first defaults.
func Default() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".bctx")
	return &Config{
		App: AppConfig{
			DataDir:  dataDir,
			LogLevel: "info",
		},
		Models: ModelsConfig{
			Directory: filepath.Join(dataDir, "models"),
		},
		Monitor: MonitorConfig{DefaultInterval: "30s"},
		Monitoring: MonitoringConfig{
			PollInterval: "30s", MaxEventQueue: 1024, AnalysisDebounce: "2s",
			ReconnectBackoff: "5s", MaxReconnectAttempts: 10,
			AlertRiskDelta: 10, AlertNewSignal: true, AlertNewPattern: true,
			SessionRetention: 10000,
		},
		Reports: ReportsConfig{DefaultFormat: "pdf"},
		Acquisition: AcquisitionConfig{
			// Default to the real Esplora provider so a fresh install queries
			// the live chain. The deterministic "fake" provider remains
			// available for tests/offline demos via explicit config.
			Provider: "esplora", MaxWorkers: 4, PageSize: 100, BatchSize: 500,
			RequestTimeout: "20s", MaxRetries: 4,
			InitialBackoff: "200ms", MaxBackoff: "5s", RequestsPerSecond: 5,
		},
		Network: NetworkConfig{
			AcquisitionEnabled: true,
			Mode:               ModeOnline,
			// Blockstream public Esplora instance (fast, no key); overridable.
			ExplorerEndpoint: "https://blockstream.info/api",
		},
		LLM: LLMConfig{
			Enabled:  false,
			Endpoint: "http://localhost:11434",
			Model:    "gemma3:1b",
		},
	}
}

// DefaultPath returns the standard config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	// Follow XDG-ish layout: ~/.config/bctx/config.toml
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bctx", "config.toml")
	}
	return filepath.Join(home, ".config", "bctx", "config.toml")
}

// Load reads configuration from path, falling back to defaults for any missing
// values. A non-existent file is not an error — defaults are returned.
func Load(path string) (*Config, error) {
	cfg := Default()
	cfg.path = path

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.path = path
	cfg.normalize()
	return cfg, nil
}

// normalize expands "~" and fills derived defaults after unmarshalling.
func (c *Config) normalize() {
	home, _ := os.UserHomeDir()
	expand := func(p string) string {
		if len(p) >= 2 && p[:2] == "~/" {
			return filepath.Join(home, p[2:])
		}
		return p
	}
	c.App.DataDir = expand(c.App.DataDir)
	c.Models.Directory = expand(c.Models.Directory)
	if c.App.DataDir == "" {
		c.App.DataDir = filepath.Join(home, ".bctx")
	}
	if c.Models.Directory == "" {
		c.Models.Directory = filepath.Join(c.App.DataDir, "models")
	}
	if c.Network.Mode == "" {
		c.Network.Mode = ModeOnline
	}
}

// Save writes the configuration to its path, creating parent directories.
func (c *Config) Save() error {
	if c.path == "" {
		c.path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(c.path)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

// Path returns the config file path.
func (c *Config) Path() string { return c.path }

// AcquisitionAllowed reports whether acquisition may run given mode + flag.
func (c *Config) AcquisitionAllowed() bool {
	if c.Network.Mode == ModeAirgap || c.Network.Mode == ModeOffline {
		return false
	}
	return c.Network.AcquisitionEnabled
}
