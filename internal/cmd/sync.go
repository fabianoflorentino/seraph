package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/catalog"
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

			services := make([]state.Service, 0, len(cat.Services))
			for _, entry := range cat.Services {
				services = append(services, state.Service{
					Slug:        entry.Slug,
					Name:        entry.Name,
					Category:    string(entry.Category),
					OfficialURL: entry.OfficialURL,
					Regions:     entry.RegionList(),
					Status:      state.StatusUnknown,
				})
			}

			snapshot := state.NewSnapshot(time.Now(), services)
			snapshot.Sources = state.Sources{Failed: len(services)}

			outPath := filepath.Join(cfg.OutDir, "state.json")
			if err := state.WriteAtomic(outPath, snapshot); err != nil {
				return err
			}

			logger.Info("snapshot written", "path", outPath, "services", len(services))

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s with %d services\n", outPath, len(services))

			return err
		},
	}

	cmd.Flags().StringVar(&catalogDir, "catalog", defaultCatalogDir, "directory containing catalog packs")

	return cmd
}
