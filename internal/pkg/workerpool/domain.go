package workerpool

import (
	"fmt"
	"regexp"
	"strings"
)

// DomainExtractor extracts domains from different types of traces
type DomainExtractor struct {
	emailRegex *regexp.Regexp
	urlRegex   *regexp.Regexp
}

// NewDomainExtractor creates a new domain extractor
func NewDomainExtractor() *DomainExtractor {
	return &DomainExtractor{
		emailRegex: regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})$`),
		urlRegex:   regexp.MustCompile(`^https?://([a-zA-Z0-9.-]+(?:\.[a-zA-Z]{2,})?(?::[0-9]+)?)`),
	}
}

// ExtractDomain extracts the domain from a task payload
func (de *DomainExtractor) ExtractDomain(task *Task) (string, error) {
	if task == nil || task.Payload == nil {
		return "", fmt.Errorf("task or payload is nil")
	}

	// Convert payload to string for analysis
	payloadStr := fmt.Sprintf("%v", task.Payload)

	// Try to extract domain from email
	if domain := de.extractEmailDomain(payloadStr); domain != "" {
		return domain, nil
	}

	// Try to extract domain from URL
	if domain := de.extractURLDomain(payloadStr); domain != "" {
		return domain, nil
	}

	// Try to extract domain from domain-only string
	if domain := de.extractDomainOnly(payloadStr); domain != "" {
		return domain, nil
	}

	// If no domain found, return a default domain for rate limiting
	return "default", nil
}

// extractEmailDomain extracts domain from email addresses
func (de *DomainExtractor) extractEmailDomain(input string) string {
	matches := de.emailRegex.FindStringSubmatch(input)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// extractURLDomain extracts domain from URLs
func (de *DomainExtractor) extractURLDomain(input string) string {
	matches := de.urlRegex.FindStringSubmatch(input)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// extractDomainOnly extracts domain from domain-only strings
func (de *DomainExtractor) extractDomainOnly(input string) string {
	// Simple domain validation regex
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	if domainRegex.MatchString(input) {
		return strings.ToLower(input)
	}
	return ""
}

// ValidateDomain validates if a domain string is properly formatted
func (de *DomainExtractor) ValidateDomain(domain string) bool {
	if domain == "" || domain == "default" {
		return true // Allow default domain
	}

	// Basic domain validation
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}
