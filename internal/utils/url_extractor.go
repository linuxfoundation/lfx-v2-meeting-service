// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/url"
	"regexp"
	"strings"
)

// urlPattern matches HTTP and HTTPS URLs
// Pattern explanation:
// - https?:// - matches http:// or https://
// - [^\s<>"]+ - matches one or more characters that are not whitespace, <, >, or "
var urlPattern = regexp.MustCompile(`https?://[^\s<>"]+`)

// ExtractURLs extracts all HTTP/HTTPS URLs from the given text.
// It returns a deduplicated list of URLs in the order they appear.
// URLs are matched exactly as they appear in the text, including query parameters and fragments.
func ExtractURLs(text string) []string {
	if text == "" {
		return []string{}
	}

	// Find all URL matches
	matches := urlPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return []string{}
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	urls := make([]string, 0, len(matches))

	for _, url := range matches {
		// Clean up URLs that might have trailing punctuation
		url = cleanTrailingPunctuation(url)

		// Skip if we've already seen this exact URL
		if seen[url] {
			continue
		}

		seen[url] = true
		urls = append(urls, url)
	}

	return urls
}

// cleanTrailingPunctuation removes common trailing punctuation that might be captured
// by the regex but shouldn't be part of the URL (e.g., periods, commas at end of sentences)
// Note: This may incorrectly strip legitimate closing delimiters from URLs like Wikipedia
// disambiguation pages. For meeting description extraction, this trade-off is acceptable.
func cleanTrailingPunctuation(url string) string {
	// Common trailing punctuation to remove
	trailingChars := []string{".", ",", "!", "?", ";", ":", ")", "]", "}"}

	for {
		trimmed := false
		for _, char := range trailingChars {
			if strings.HasSuffix(url, char) {
				url = strings.TrimSuffix(url, char)
				trimmed = true
				break
			}
		}
		// If no trailing punctuation was removed, we're done
		if !trimmed {
			break
		}
	}

	return url
}

// ExtractDomain extracts the domain (host) from a URL string.
// Returns the domain without protocol, or the original URL if parsing fails.
// Examples:
//   - "https://example.com/path" -> "example.com"
//   - "http://subdomain.example.com:8080/path" -> "subdomain.example.com"
//   - "https://github.com/org/repo" -> "github.com"
func ExtractDomain(urlString string) string {
	parsed, err := url.Parse(urlString)
	if err != nil {
		// If parsing fails, return the original URL
		return urlString
	}

	// Return just the hostname (without port if present)
	if parsed.Hostname() != "" {
		return parsed.Hostname()
	}

	// Fallback to the full host (includes port if present)
	if parsed.Host != "" {
		return parsed.Host
	}

	// If we couldn't extract the host, return the original URL
	return urlString
}
