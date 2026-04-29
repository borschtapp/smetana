package utils

import (
	"net/url"

	"github.com/PuerkitoBio/purell"
)

const normalizeFlags = purell.FlagsUsuallySafeGreedy |
	purell.FlagRemoveWWW | purell.FlagRemoveDuplicateSlashes | purell.FlagSortQuery | purell.FlagRemoveFragment

func NormalizeURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}
	normalized, err := purell.NormalizeURLString(rawURL, normalizeFlags)
	if err != nil {
		return rawURL
	}
	return normalized
}

func BaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return purell.NormalizeURL(u, normalizeFlags)
}
