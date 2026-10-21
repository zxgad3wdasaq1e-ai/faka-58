package mailbrand

import (
	"context"
	"strings"
)

// NormalizeName replaces retired built-in names while preserving custom brands.
func NormalizeName(name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "XOPEN AI", "XIOEN AI", "Dujiao-Next", "Dujiao Next":
		return "faka-58"
	default:
		return name
	}
}

// Brand describes the customer-visible identity used by one email.
type Brand struct {
	SiteName string
	SiteURL  string
	FromName string
	ReplyTo  string
}

// Scope identifies the storefront whose brand must be used.
// A nil ResellerID represents the main storefront.
type Scope struct {
	ResellerID *uint
	Host       string
}

// Resolver resolves a safe email brand for a storefront scope.
type Resolver interface {
	ResolveEmailBrand(ctx context.Context, scope Scope) (Brand, error)
}

// ResellerFallback builds a tenant-safe brand without consulting global
// settings. It is used when a reseller has not configured a site name yet.
func ResellerFallback(host string) Brand {
	normalized := strings.TrimSpace(host)
	siteURL := ""
	if normalized != "" {
		siteURL = "https://" + normalized
	}
	return Brand{
		SiteName: normalized,
		SiteURL:  siteURL,
		FromName: normalized,
	}
}
