package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/collector"
	"github.com/fabianoflorentino/seraph/internal/state"
)

const defaultCatalogDir = "catalog/packs"

func newSyncCommand(opts *rootOptions) *cobra.Command {
	var catalogDir string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Collect catalog services and write the dashboard snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}

			logger, err := newLogger(cfg.LogLevel, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			cat, err := catalog.Load(os.DirFS(catalogDir), ".")
			if err != nil {
				return err
			}

			observer := collector.New(
				opts.httpClient,
				collector.WithConcurrency(cfg.Concurrency),
				collector.WithRetries(cfg.Retries),
				collector.WithTimeout(cfg.Timeout.Duration()),
			)

			services := observer.Collect(cmd.Context(), cat.Services)

			var sources state.Sources

			for _, service := range services {
				if service.Error != "" {
					sources.Failed++
					logger.Warn("service collection failed", "service", service.Slug, "error", service.Error)

					continue
				}

				sources.OK++
			}

			outPath := filepath.Join(cfg.OutDir, "state.json")
			snapshot := state.NewSnapshot(time.Now(), services)
			snapshot.Sources = sources

			if previous, err := state.Read(outPath); err == nil {
				snapshot.Services = state.MergeHistory(previous, snapshot.Services, state.DefaultHistoryLimit)
			} else if !errors.Is(err, fs.ErrNotExist) {
				logger.Warn("could not read previous snapshot", "error", err)
			}

			if err := state.WriteAtomic(outPath, snapshot); err != nil {
				return err
			}

			logger.Info(
				"snapshot written",
				"path", outPath,
				"services", len(services),
				"ok", sources.OK,
				"failed", sources.Failed,
			)

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s with %d services\n", outPath, len(services))

			return err
		},
	}

	cmd.Flags().StringVar(&catalogDir, "catalog", defaultCatalogDir, "directory containing catalog packs")

	return cmd
}
