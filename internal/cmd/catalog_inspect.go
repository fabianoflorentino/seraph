package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/fabianoflorentino/seraph/internal/catalog"
	"github.com/fabianoflorentino/seraph/internal/discovery"
)

var discoverySlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func newCatalogInspectCommand(opts *rootOptions) *cobra.Command {
	var (
		slug        string
		name        string
		category    string
		officialURL string
		component   string
		asJSON      bool
	)

	cmd := &cobra.Command{
		Use:   "inspect <domain|url>",
		Short: "Discover and validate status sources for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inspector := discovery.New(opts.httpClient)
			result, err := inspector.Inspect(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			best, ok := bestCandidate(result.Candidates)
			if !ok {
				return errors.New("no machine-readable status source found")
			}

			serviceName := strings.TrimSpace(name)
			if serviceName == "" {
				serviceName = strings.TrimSpace(best.Name)
			}
			if serviceName == "" {
				serviceName = strings.TrimPrefix(result.Target, "status.")
			}

			serviceSlug := strings.TrimSpace(slug)
			if serviceSlug == "" {
				serviceSlug = slugify(serviceName)
			}

			official := strings.TrimSpace(officialURL)
			if official == "" {
				official = "https://" + strings.TrimPrefix(result.Target, "status.")
			}

			if strings.TrimSpace(category) == "" {
				category = "cloud"
			}

			sourceURL := best.URL
			if best.Type == "statuspage" && best.StatusURL != "" {
				sourceURL = strings.TrimRight(best.StatusURL, "/") + "/api/v2/summary.json"
			}

			service := catalog.Service{
				Slug:        serviceSlug,
				Name:        serviceName,
				Category:    catalog.Category(category),
				OfficialURL: official,
				StatusURL:   best.StatusURL,
				Regions:     map[string]catalog.Level{catalog.GlobalRegion: catalog.LevelPrimary},
				Source: catalog.Source{
					Type:      catalog.SourceType(best.Type),
					URL:       sourceURL,
					Component: component,
					Format:    best.Format,
				},
			}

			draft, err := yaml.Marshal(service)
			if err != nil {
				return fmt.Errorf("encode draft: %w", err)
			}

			if asJSON {
				output := struct {
					Result discovery.Result `json:"result"`
					Draft  string           `json:"draft_yaml"`
				}{Result: result, Draft: string(draft)}

				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(output)
			}

			writer := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(writer, "target: %s\n", result.Target)
			for _, candidate := range result.Candidates {
				format := candidate.Type
				if candidate.Format != "" {
					format += "/" + candidate.Format
				}

				_, _ = fmt.Fprintf(writer, "- %s (%s, %dms)\n", candidate.URL, format, candidate.LatencyMS)
			}
			_, _ = fmt.Fprintf(writer, "\ndraft:\n%s", draft)

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&slug, "slug", "", "override the generated slug")
	flags.StringVar(&name, "name", "", "override the display name")
	flags.StringVar(&category, "category", "cloud", "service category")
	flags.StringVar(&officialURL, "official-url", "", "override the official service URL")
	flags.StringVar(&component, "component", "", "status page component to scope")
	flags.BoolVar(&asJSON, "json", false, "emit JSON with candidates and draft YAML")

	return cmd
}

func bestCandidate(candidates []discovery.Candidate) (discovery.Candidate, bool) {
	rank := map[string]int{"statuspage": 0, "statusio": 1, "json": 2, "rss": 3, "html": 99}
	bestIndex := -1

	for index, candidate := range candidates {
		if candidate.Type == "html" {
			continue
		}

		if bestIndex == -1 || rank[candidate.Type] < rank[candidates[bestIndex].Type] {
			bestIndex = index
		}
	}

	if bestIndex == -1 {
		return discovery.Candidate{}, false
	}

	return candidates[bestIndex], true
}

func slugify(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = discoverySlugPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		return "service"
	}

	return slug
}
