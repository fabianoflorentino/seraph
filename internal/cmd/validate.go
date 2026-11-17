package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/catalog"
)

func newValidateCommand(opts *rootOptions) *cobra.Command {
	var catalogDir string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the catalog without contacting any service",
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

			for _, service := range cat.Services {
				logger.Debug("catalog service",
					"slug", service.Slug,
					"category", service.Category,
					"source", service.Source.Type,
					"regions", strings.Join(service.RegionList(), ","),
				)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "catalog is valid: %d packs, %d services\n",
				len(cat.Packs), len(cat.Services))

			return err
		},
	}

	cmd.Flags().StringVar(&catalogDir, "catalog", defaultCatalogDir, "directory containing catalog packs")

	return cmd
}
