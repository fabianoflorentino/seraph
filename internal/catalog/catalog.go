package catalog

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
)

// GlobalRegion is the reserved region code for worldwide services.
const GlobalRegion = "global"

// Level describes how relevant a service is for a region.
type Level string

const (
	// LevelPrimary marks a service shown by default for the region.
	LevelPrimary Level = "primary"
	// LevelSecondary marks a service shown only when the user expands the region.
	LevelSecondary Level = "secondary"
)

// Category groups services for dashboard filtering.
type Category string

const (
	// CategoryBanking covers banks and financial institutions.
	CategoryBanking Category = "banking"
	// CategoryPayments covers payment networks and wallets.
	CategoryPayments Category = "payments"
	// CategoryTelecom covers carriers and internet providers.
	CategoryTelecom Category = "telecom"
	// CategoryMessaging covers chat and messaging services.
	CategoryMessaging Category = "messaging"
	// CategorySocial covers social networks.
	CategorySocial Category = "social"
	// CategoryCloud covers cloud providers and developer platforms.
	CategoryCloud Category = "cloud"
	// CategoryStreaming covers video, music, and streaming services.
	CategoryStreaming Category = "streaming"
	// CategoryGaming covers game platforms and online games.
	CategoryGaming Category = "gaming"
	// CategoryTransport covers mobility and delivery platforms.
	CategoryTransport Category = "transport"
	// CategoryGov covers government services.
	CategoryGov Category = "gov"
	// CategoryEcommerce covers online marketplaces and stores.
	CategoryEcommerce Category = "ecommerce"
)

var categories = []Category{
	CategoryBanking,
	CategoryPayments,
	CategoryTelecom,
	CategoryMessaging,
	CategorySocial,
	CategoryCloud,
	CategoryStreaming,
	CategoryGaming,
	CategoryTransport,
	CategoryGov,
	CategoryEcommerce,
}

// Categories returns the supported categories in display order.
func Categories() []Category {
	out := make([]Category, len(categories))
	copy(out, categories)

	return out
}

func (c Category) valid() bool {
	for _, known := range categories {
		if c == known {
			return true
		}
	}

	return false
}

// SourceType identifies how a service is observed.
type SourceType string

const (
	// SourceStatuspage reads an Atlassian Statuspage summary.
	SourceStatuspage SourceType = "statuspage"
	// SourceStatusIO reads a status.io status document.
	SourceStatusIO SourceType = "statusio"
	// SourceRSS reads an incident RSS feed.
	SourceRSS SourceType = "rss"
	// SourceProbe checks a health endpoint directly.
	SourceProbe SourceType = "probe"
)

func (s SourceType) valid() bool {
	switch s {
	case SourceStatuspage, SourceStatusIO, SourceRSS, SourceProbe:
		return true
	default:
		return false
	}
}

// Source points at the endpoint used to observe a service.
type Source struct {
	Type      SourceType `yaml:"type"`
	URL       string     `yaml:"url"`
	Component string     `yaml:"component,omitempty"`
}

// Service is one monitored service entry in a catalog pack.
type Service struct {
	Slug        string           `yaml:"slug"`
	Name        string           `yaml:"name"`
	Category    Category         `yaml:"category"`
	OfficialURL string           `yaml:"official_url,omitempty"`
	Regions     map[string]Level `yaml:"regions"`
	Source      Source           `yaml:"source"`
}

// RegionList returns the service region codes in sorted order.
func (s Service) RegionList() []string {
	out := make([]string, 0, len(s.Regions))
	for region := range s.Regions {
		out = append(out, region)
	}

	sort.Strings(out)

	return out
}

// Pack is one YAML file of services, usually scoped to a region or audience.
type Pack struct {
	Version  int       `yaml:"version"`
	Services []Service `yaml:"services"`
}

// Catalog is the merged, validated view of every loaded pack.
type Catalog struct {
	Packs    []Pack
	Services []Service
}

// ErrInvalidCatalog is returned when a pack fails validation.
var ErrInvalidCatalog = errors.New("invalid catalog")

// FieldError describes a single catalog validation failure.
type FieldError struct {
	File    string
	Slug    string
	Field   string
	Message string
}

// Error implements error.
func (e *FieldError) Error() string {
	location := e.Field
	if e.Slug != "" {
		location = e.Slug + "." + e.Field
	}

	if e.File != "" {
		location = e.File + ": " + location
	}

	return location + ": " + e.Message
}

// Unwrap reports the sentinel so callers can use errors.Is.
func (e *FieldError) Unwrap() error { return ErrInvalidCatalog }

var (
	slugPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	regionPattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

func (p Pack) validate(file string) error {
	var errs []error

	if p.Version != 1 {
		errs = append(errs, &FieldError{
			File:    file,
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d, want 1", p.Version),
		})
	}

	seen := make(map[string]struct{}, len(p.Services))

	for index, service := range p.Services {
		for _, err := range service.validate() {
			var fieldErr *FieldError

			if errors.As(err, &fieldErr) {
				fieldErr.File = file
				if fieldErr.Slug == "" {
					fieldErr.Slug = fmt.Sprintf("services[%d]", index)
				}
			}

			errs = append(errs, err)
		}

		if service.Slug == "" {
			continue
		}

		if _, ok := seen[service.Slug]; ok {
			errs = append(errs, &FieldError{
				File:    file,
				Slug:    service.Slug,
				Field:   "slug",
				Message: "duplicate slug in pack",
			})
		}

		seen[service.Slug] = struct{}{}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s Service) validate() []error {
	var errs []error

	add := func(field, message string) {
		errs = append(errs, &FieldError{Slug: s.Slug, Field: field, Message: message})
	}

	switch {
	case s.Slug == "":
		add("slug", "must not be empty")
	case !slugPattern.MatchString(s.Slug):
		add("slug", "must be lower-kebab-case (a-z, 0-9, single dashes)")
	}

	if name := s.Name; name == "" {
		add("name", "must not be empty")
	} else if len(name) > maxNameLength {
		add("name", fmt.Sprintf("must be at most %d characters", maxNameLength))
	}

	if !s.Category.valid() {
		add("category", fmt.Sprintf("unsupported category %q", s.Category))
	}

	if s.OfficialURL != "" {
		if err := validateHTTPS(s.OfficialURL); err != nil {
			add("official_url", err.Error())
		}
	}

	if len(s.Regions) == 0 {
		add("regions", "must declare at least one region")
	}

	primary := 0

	for region, level := range s.Regions {
		if region != GlobalRegion && !regionPattern.MatchString(region) {
			add("regions", fmt.Sprintf("invalid region %q, use global or ISO 3166-1 alpha-2", region))
		}

		switch level {
		case LevelPrimary:
			primary++
		case LevelSecondary:
		default:
			add("regions", fmt.Sprintf("invalid level %q for region %s", level, region))
		}
	}

	if len(s.Regions) > 0 && primary == 0 {
		add("regions", "must declare at least one primary region")
	}

	if !s.Source.Type.valid() {
		add("source.type", fmt.Sprintf("unsupported source type %q", s.Source.Type))
	}

	if s.Source.URL == "" {
		add("source.url", "must not be empty")
	} else if err := validateHTTPS(s.Source.URL); err != nil {
		add("source.url", err.Error())
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

const maxNameLength = 80

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
