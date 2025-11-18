// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "no URLs",
			input:    "This is a meeting agenda with no links",
			expected: []string{},
		},
		{
			name:     "single HTTP URL",
			input:    "Check out http://example.com for details",
			expected: []string{"http://example.com"},
		},
		{
			name:     "single HTTPS URL",
			input:    "Check out https://example.com for details",
			expected: []string{"https://example.com"},
		},
		{
			name:     "multiple URLs",
			input:    "Visit https://example.com and https://test.org for more info",
			expected: []string{"https://example.com", "https://test.org"},
		},
		{
			name:     "duplicate URLs - should deduplicate",
			input:    "Visit https://example.com twice: https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "URL with path",
			input:    "See https://example.com/path/to/resource",
			expected: []string{"https://example.com/path/to/resource"},
		},
		{
			name:     "URL with query parameters",
			input:    "Link: https://example.com/page?param1=value1&param2=value2",
			expected: []string{"https://example.com/page?param1=value1&param2=value2"},
		},
		{
			name:     "URL with fragment",
			input:    "Jump to https://example.com/page#section",
			expected: []string{"https://example.com/page#section"},
		},
		{
			name:     "URL with query and fragment",
			input:    "See https://example.com/page?id=123#top",
			expected: []string{"https://example.com/page?id=123#top"},
		},
		{
			name:     "URL at end of sentence with period",
			input:    "Visit our site at https://example.com.",
			expected: []string{"https://example.com"},
		},
		{
			name:     "URL at end of sentence with comma",
			input:    "Check https://example.com, then proceed",
			expected: []string{"https://example.com"},
		},
		{
			name:     "URL in parentheses",
			input:    "Our website (https://example.com) has details",
			expected: []string{"https://example.com"},
		},
		{
			name:     "URL in brackets",
			input:    "See [https://example.com] for info",
			expected: []string{"https://example.com"},
		},
		{
			name:     "multiple URLs with various punctuation",
			input:    "Links: https://one.com, https://two.org; https://three.net!",
			expected: []string{"https://one.com", "https://two.org", "https://three.net"},
		},
		{
			name:     "URLs in markdown-like format",
			input:    "Check [link](https://example.com) and visit https://test.org",
			expected: []string{"https://example.com", "https://test.org"},
		},
		{
			name:     "URL with port",
			input:    "Local server: http://localhost:8080/api",
			expected: []string{"http://localhost:8080/api"},
		},
		{
			name:     "multiple lines with URLs",
			input:    "Line 1: https://example.com\nLine 2: https://test.org\nLine 3: no link",
			expected: []string{"https://example.com", "https://test.org"},
		},
		{
			name:     "URLs with different domains",
			input:    "https://github.com/repo and https://gitlab.com/project",
			expected: []string{"https://github.com/repo", "https://gitlab.com/project"},
		},
		{
			name:     "URL with underscores and hyphens",
			input:    "https://my-site_example.com/my_path-here",
			expected: []string{"https://my-site_example.com/my_path-here"},
		},
		{
			name: "realistic meeting agenda",
			input: `Meeting Agenda:
1. Review project status: https://github.com/org/project/issues
2. Discuss documentation at https://docs.example.com/guide
3. Check deployment: https://app.example.com/dashboard?env=prod
4. No link here
5. Final notes at https://wiki.example.com/notes`,
			expected: []string{
				"https://github.com/org/project/issues",
				"https://docs.example.com/guide",
				"https://app.example.com/dashboard?env=prod",
				"https://wiki.example.com/notes",
			},
		},
		{
			name:     "URL with special characters in query",
			input:    "Search: https://example.com/search?q=hello+world&lang=en",
			expected: []string{"https://example.com/search?q=hello+world&lang=en"},
		},
		{
			name:     "FTP URLs should not be captured",
			input:    "FTP link: ftp://example.com/file and HTTP: https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "mixed case URLs - regex preserves URL case",
			input:    "Visit https://EXAMPLE.COM and https://example.com",
			expected: []string{"https://EXAMPLE.COM", "https://example.com"},
		},
		{
			name:     "URL with trailing slash",
			input:    "Homepage: https://example.com/ and page https://example.com/page/",
			expected: []string{"https://example.com/", "https://example.com/page/"},
		},
		{
			name:     "URL embedded in HTML-like tags",
			input:    "<a href=\"https://example.com\">Link</a>",
			expected: []string{"https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractURLs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanTrailingPunctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no trailing punctuation",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "trailing period",
			input:    "https://example.com.",
			expected: "https://example.com",
		},
		{
			name:     "trailing comma",
			input:    "https://example.com,",
			expected: "https://example.com",
		},
		{
			name:     "trailing exclamation",
			input:    "https://example.com!",
			expected: "https://example.com",
		},
		{
			name:     "trailing question mark - should be removed",
			input:    "https://example.com?",
			expected: "https://example.com",
		},
		{
			name:     "trailing semicolon",
			input:    "https://example.com;",
			expected: "https://example.com",
		},
		{
			name:     "trailing colon",
			input:    "https://example.com:",
			expected: "https://example.com",
		},
		{
			name:     "trailing closing paren",
			input:    "https://example.com)",
			expected: "https://example.com",
		},
		{
			name:     "trailing closing bracket",
			input:    "https://example.com]",
			expected: "https://example.com",
		},
		{
			name:     "trailing closing brace",
			input:    "https://example.com}",
			expected: "https://example.com",
		},
		{
			name:     "multiple trailing punctuation",
			input:    "https://example.com.,;",
			expected: "https://example.com",
		},
		{
			name:     "URL with query param - don't remove question mark from middle",
			input:    "https://example.com/page?id=123",
			expected: "https://example.com/page?id=123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanTrailingPunctuation(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple HTTPS URL",
			input:    "https://example.com",
			expected: "example.com",
		},
		{
			name:     "HTTP URL",
			input:    "http://example.com",
			expected: "example.com",
		},
		{
			name:     "URL with path",
			input:    "https://example.com/path/to/resource",
			expected: "example.com",
		},
		{
			name:     "URL with subdomain",
			input:    "https://subdomain.example.com",
			expected: "subdomain.example.com",
		},
		{
			name:     "URL with multiple subdomains",
			input:    "https://api.staging.example.com/v1/endpoint",
			expected: "api.staging.example.com",
		},
		{
			name:     "URL with port",
			input:    "http://example.com:8080/path",
			expected: "example.com",
		},
		{
			name:     "URL with subdomain and port",
			input:    "http://subdomain.example.com:3000",
			expected: "subdomain.example.com",
		},
		{
			name:     "URL with query parameters",
			input:    "https://example.com/search?q=test&page=1",
			expected: "example.com",
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/page#section",
			expected: "example.com",
		},
		{
			name:     "localhost URL",
			input:    "http://localhost:8080/api",
			expected: "localhost",
		},
		{
			name:     "IP address URL",
			input:    "http://192.168.1.1:80/admin",
			expected: "192.168.1.1",
		},
		{
			name:     "complex real-world URL",
			input:    "https://app.staging.lfx.dev/project/kubernetes/meetings?tab=upcoming",
			expected: "app.staging.lfx.dev",
		},
		{
			name:     "URL with www",
			input:    "https://www.example.com/page",
			expected: "www.example.com",
		},
		{
			name:     "invalid URL - returns original",
			input:    "not-a-valid-url",
			expected: "not-a-valid-url",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
