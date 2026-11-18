package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/catalog"
)

func newCatalogCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage the service catalog",
	}

	cmd.AddCommand(
		newCatalogAddCommand(),
		newCatalogInspectCommand(opts),
		newCatalogAuditCommand(opts),
	)

	return cmd
}

func newCatalogAddCommand() *cobra.Command {
	var (
		pack        string
		slug        string
		name        string
		category    string
		officialURL string
		statusURL   string
		component   string
		format      string
		sourceType  string
		sourceURL   string
		regions     []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a service to a catalog pack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch {
			case slug == "":
				return fmt.Errorf("--slug is required")
			case name == "":
				return fmt.Errorf("--name is required")
			case category == "":
				return fmt.Errorf("--category is required")
			case sourceType == "":
				return fmt.Errorf("--source-type is required")
			case sourceURL == "":
				return fmt.Errorf("--source-url is required")
			}

			regionMap := make(map[string]catalog.Level, len(regions))
			for _, region := range regions {
				regionMap[region] = catalog.LevelPrimary
			}

			service := catalog.Service{
				Slug:        slug,
				Name:        name,
				Category:    catalog.Category(category),
				OfficialURL: officialURL,
				StatusURL:   statusURL,
				Regions:     regionMap,
				Source: catalog.Source{
					Type:      catalog.SourceType(sourceType),
					URL:       sourceURL,
					Component: component,
					Format:    format,
				},
			}

			if err := catalog.AddService(pack, service); err != nil {
				return err
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "added %s to %s\n", slug, pack)

			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&pack, "pack", "catalog/packs/global.yaml", "catalog pack to update")
	flags.StringVar(&slug, "slug", "", "service slug (lower-kebab-case)")
	flags.StringVar(&name, "name", "", "display name")
	flags.StringVar(&category, "category", "", "service category")
	flags.StringVar(&officialURL, "official-url", "", "official service URL")
	flags.StringVar(&statusURL, "status-url", "", "public status page URL")
	flags.StringVar(&sourceType, "source-type", "", "source type: statuspage, statusio, rss, json")
	flags.StringVar(&sourceURL, "source-url", "", "source endpoint URL")
	flags.StringVar(&component, "component", "", "status page component name")
	flags.StringVar(&format, "format", "", "json format: slack or stripe")
	flags.StringArrayVar(&regions, "region", []string{"global"}, "region code; repeatable")

	return cmd
}
