// Package cmd implements the seraph command line interface.
package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/config"
)

type rootOptions struct {
	configPath string
	logLevel   string
	httpClient *http.Client
}

// NewRootCommand builds the seraph command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(&rootOptions{httpClient: &http.Client{}})
}

func newRootCommand(opts *rootOptions) *cobra.Command {
	if opts.httpClient == nil {
		opts.httpClient = &http.Client{}
	}

	cmd := &cobra.Command{
		Use:   "seraph",
		Short: "Monitor official status sources and publish a static dashboard",
		Long: `seraph collects official status pages and feeds for a curated catalog of
services, then publishes a static JSON snapshot for the dashboard.

Favorites live in the browser, so no account or database is required.`,
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.configPath, "config", "", "configuration file (default "+config.DefaultPath+")")
	flags.StringVar(&opts.logLevel, "log-level", "", "log level: debug, info, warn, error")

	cmd.AddCommand(
		newVersionCommand(),
		newCatalogCommand(),
		newSyncCommand(opts),
		newValidateCommand(opts),
	)

	return cmd
}

// Execute runs the root command.
func Execute() error {
	return NewRootCommand().Execute()
}

func loadConfig(opts *rootOptions) (*config.Config, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, err
	}

	if opts.logLevel != "" {
		cfg.LogLevel = strings.ToLower(opts.logLevel)

		if err := cfg.Validate(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func newLogger(level string, w io.Writer) (*slog.Logger, error) {
	var parsed slog.Level

	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", level, err)
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: parsed})), nil
}
