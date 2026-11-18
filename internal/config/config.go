package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultPath is the configuration file loaded when no path is provided.
const DefaultPath = "config/seraph.yml"

const (
	minTimeout     = time.Second
	maxTimeout     = 60 * time.Second
	minConcurrency = 1
	maxConcurrency = 128
	maxRetries     = 10
)

// Duration is a YAML- and environment-friendly time.Duration.
type Duration time.Duration

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration { return time.Duration(d) }

// String implements fmt.Stringer.
func (d Duration) String() string { return time.Duration(d).String() }

// UnmarshalYAML decodes values such as "8s" or "500ms".
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("decode duration: %w", err)
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}

	*d = Duration(parsed)

	return nil
}

// MarshalYAML encodes the duration using Go duration syntax.
func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }

// Config holds the runtime settings for a sync run.
type Config struct {
	OutDir      string   `yaml:"out_dir"`
	BaseURL     string   `yaml:"base_url"`
	Timeout     Duration `yaml:"timeout"`
	Concurrency int      `yaml:"concurrency"`
	Retries     int      `yaml:"retries"`
	LogLevel    string   `yaml:"log_level"`
}

// Default returns the built-in configuration.
func Default() *Config {
	return &Config{
		OutDir:      "site/static/data",
		BaseURL:     "",
		Timeout:     Duration(8 * time.Second),
		Concurrency: 12,
		Retries:     2,
		LogLevel:    "info",
	}
}

// Load reads the configuration from path, applies SERAPH_* environment
// overrides, and validates the result. An empty path resolves to
// DefaultPath, which is optional: a missing default file falls back to
// Default. An explicitly provided path must exist.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = DefaultPath
	}

	//nolint:gosec // the path comes from the operator or configuration, not from user input.
	data, err := os.ReadFile(path)

	switch {
	case err == nil:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)

		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("decode config %s: %w", path, err)
		}
	case errors.Is(err, fs.ErrNotExist) && path == DefaultPath:
		// The default file is optional; defaults are already in place.
	default:
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate reports invalid runtime settings.
func (c *Config) Validate() error {
	c.OutDir = strings.TrimSpace(c.OutDir)

	if c.OutDir == "" {
		return errors.New("config: out_dir must not be empty")
	}

	if c.BaseURL != "" {
		if err := validateHTTPS(c.BaseURL); err != nil {
			return fmt.Errorf("config: base_url: %w", err)
		}
	}

	timeout := c.Timeout.Duration()
	if timeout < minTimeout || timeout > maxTimeout {
		return fmt.Errorf("config: timeout must be between %s and %s", minTimeout, maxTimeout)
	}

	if c.Concurrency < minConcurrency || c.Concurrency > maxConcurrency {
		return fmt.Errorf("config: concurrency must be between %d and %d", minConcurrency, maxConcurrency)
	}

	if c.Retries < 0 || c.Retries > maxRetries {
		return fmt.Errorf("config: retries must be between 0 and %d", maxRetries)
	}

	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
		c.LogLevel = strings.ToLower(c.LogLevel)
	default:
		return fmt.Errorf("config: unsupported log_level %q", c.LogLevel)
	}

	return nil
}

func (c *Config) applyEnv() error {
	if v := os.Getenv("SERAPH_OUT_DIR"); v != "" {
		c.OutDir = v
	}

	if v := os.Getenv("SERAPH_BASE_URL"); v != "" {
		c.BaseURL = v
	}

	if v := os.Getenv("SERAPH_TIMEOUT"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("config: SERAPH_TIMEOUT: %w", err)
		}

		c.Timeout = Duration(parsed)
	}

	if v := os.Getenv("SERAPH_CONCURRENCY"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("config: SERAPH_CONCURRENCY: %w", err)
		}

		c.Concurrency = parsed
	}

	if v := os.Getenv("SERAPH_RETRIES"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("config: SERAPH_RETRIES: %w", err)
		}

		c.Retries = parsed
	}

	if v := os.Getenv("SERAPH_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}

	return nil
}

func validateHTTPS(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("must be an absolute https URL")
	}

	return nil
}
