package security

import (
	"regexp"
	"strings"
)

// MaskStrategy defines how to mask sensitive data
type MaskStrategy int

const (
	MaskNone    MaskStrategy = iota // No masking
	MaskFull                        // Full mask: ********
	MaskPartial                     // Partial mask: abc••••xyz
)

// Masker handles sensitive data masking
type Masker struct {
	sensitiveFields map[string]MaskStrategy
	patterns        []*regexp.Regexp
}

// NewMasker creates a new masker with default sensitive fields
func NewMasker() *Masker {
	m := &Masker{
		sensitiveFields: map[string]MaskStrategy{
			"cf_api_token":      MaskPartial,
			"server_password":   MaskFull,
			"password":          MaskFull,
			"api_token":         MaskPartial,
			"token":             MaskPartial,
			"secret":            MaskFull,
			"key":               MaskPartial,
			"access_key":        MaskPartial,
			"secret_key":        MaskPartial,
		},
	}
	
	// Pre-compile regex patterns for common sensitive patterns
	m.initPatterns()
	
	return m
}

// initPatterns initializes regex patterns for sensitive data
func (m *Masker) initPatterns() {
	// API token pattern: Long alphanumeric strings (32+ chars)
	tokenPattern := regexp.MustCompile(`\b[a-zA-Z0-9]{32,}\b`)
	
	// API key pattern: Keys in format key=value
	apiKeyPattern := regexp.MustCompile(`(cf_api_token|api_token|token|access_key|secret_key)["\s:=]+([a-zA-Z0-9_-]{16,})`)
	
	// Password pattern: password="..." or password:...
	passwordPattern := regexp.MustCompile(`(password|passwd)["\s:=]+([^\s"\'\)]{8,})`)
	
	// Bearer token pattern: Bearer <token>
	bearerPattern := regexp.MustCompile(`Bearer\s+([a-zA-Z0-9._\-+=/]{20,})`)
	
	m.patterns = []*regexp.Regexp{
		apiKeyPattern,
		passwordPattern,
		bearerPattern,
		tokenPattern,
	}
}

// Mask masks a value based on the field name
func (m *Masker) Mask(value, field string) string {
	strategy, ok := m.sensitiveFields[field]
	if !ok {
		// Check if field contains sensitive keywords
		if m.isSensitiveField(field) {
			return m.applyMask(value, MaskPartial)
		}
		return value
	}
	
	return m.applyMask(value, strategy)
}

// ApplyMask applies masking strategy to a value
func (m *Masker) applyMask(value string, strategy MaskStrategy) string {
	switch strategy {
	case MaskFull:
		return "********"
	case MaskPartial:
		return m.maskPartial(value)
	default:
		return value
	}
}

// MaskPartial masks a value partially (show first 3 and last 2 chars)
func (m *Masker) maskPartial(value string) string {
	if value == "" {
		return ""
	}
	
	// If too short, mask fully
	if len(value) <= 5 {
		return strings.Repeat("•", len(value))
	}
	
	// Show first 3 and last 2 characters
	return value[:3] + "•••••••••••••••" + value[len(value)-2:]
}

// IsSensitiveField checks if a field name contains sensitive keywords
func (m *Masker) isSensitiveField(field string) bool {
	sensitiveKeywords := []string{
		"password", "passwd", "pwd",
		"token", "api_key", "apikey", "secret",
		"key", "private", "credential",
	}
	
	lowerField := strings.ToLower(field)
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerField, keyword) {
			return true
		}
	}
	return false
}

// MaskInString masks all sensitive patterns in a string
func (m *Masker) MaskInString(text string) string {
	if text == "" {
		return text
	}
	
	result := text
	
	// Apply pattern-based masking
	for _, pattern := range m.patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			return m.maskSensitivePattern(match, pattern)
		})
	}
	
	return result
}

// maskSensitivePattern masks a matched sensitive pattern
func (m *Masker) maskSensitivePattern(match string, pattern *regexp.Regexp) string {
	// Get submatches
	submatches := pattern.FindStringSubmatch(match)
	if len(submatches) <= 1 {
		// No submatches, mask the whole match
		return m.maskPartial(match)
	}
	
	// For patterns with groups (e.g., key=value), mask only the value
	if len(submatches) > 2 {
		// submatches[0] = full match, submatches[1] = key, submatches[2] = value
		// key := submatches[1]
		value := submatches[2]
		maskedValue := m.maskPartial(value)
		
		// Reconstruct with masked value
		return strings.Replace(match, value, maskedValue, 1)
	}
	
	// Mask the first submatch (usually the value)
	maskedValue := m.maskPartial(submatches[1])
	return strings.Replace(match, submatches[1], maskedValue, 1)
}

// SetMaskStrategy sets the masking strategy for a field
func (m *Masker) SetMaskStrategy(field string, strategy MaskStrategy) {
	m.sensitiveFields[field] = strategy
}

// GetMaskStrategy returns the masking strategy for a field
func (m *Masker) GetMaskStrategy(field string) MaskStrategy {
	if strategy, ok := m.sensitiveFields[field]; ok {
		return strategy
	}
	return MaskNone
}