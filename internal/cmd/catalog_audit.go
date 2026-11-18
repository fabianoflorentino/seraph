package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/collector"
)

type auditService struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type auditOutput struct {
	Services []auditService `json:"services"`
	Total    int            `json:"total"`
	OK       int            `json:"ok"`
	Failed   int            `json:"failed"`
	Sources  map[string]int `json:"sources"`
}

func newCatalogAuditCommand(opts *rootOptions) *cobra.Command {
	var (
		catalogDir string
		asJSON     bool
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Collect every catalog source and report its health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(opts)
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
			results := observer.Collect(cmd.Context(), cat.Services)

			sources := make(map[string]int)
			services := make([]auditService, 0, len(results))
			okCount := 0

			for index, service := range results {
				source := string(cat.Services[index].Source.Type)
				sources[source]++

				entry := auditService{
					Slug:      service.Slug,
					Name:      service.Name,
					Status:    string(service.Status),
					Source:    source,
					LatencyMS: service.LatencyMS,
					Error:     service.Error,
				}
				services = append(services, entry)

				if service.Error == "" {
					okCount++
				}
			}

			output := auditOutput{
				Services: services,
				Total:    len(services),
				OK:       okCount,
				Failed:   len(services) - okCount,
				Sources:  sources,
			}

			if asJSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(output)
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(writer, "SERVICE\tSTATUS\tSOURCE\tLATENCY\tERROR")

			for _, service := range services {
				errorText := service.Error
				if errorText == "" {
					errorText = "-"
				}

				_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%dms\t%s\n",
					service.Slug, service.Status, service.Source, service.LatencyMS, errorText)
			}

			_, _ = fmt.Fprintf(writer, "\n%d services: %d ok, %d failed\n", output.Total, output.OK, output.Failed)
			_, _ = fmt.Fprint(writer, "sources:")

			sourceNames := make([]string, 0, len(sources))
			for source := range sources {
				sourceNames = append(sourceNames, source)
			}
			sort.Strings(sourceNames)

			for _, source := range sourceNames {
				_, _ = fmt.Fprintf(writer, " %s=%d", source, sources[source])
			}
			_, _ = fmt.Fprintln(writer)

			return writer.Flush()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&catalogDir, "catalog", defaultCatalogDir, "directory containing catalog packs")
	flags.BoolVar(&asJSON, "json", false, "emit machine-readable JSON")

	return cmd
}
