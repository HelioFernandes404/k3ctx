package usecases

import "github.com/systemframe/k3ctx/internal/domain"

// ListContextStatus returns all active context status entries.
func ListContextStatus(reader domain.StatusReader) ([]map[string]any, error) {
	return reader.ListContextStatus()
}

// ValidateContextNetwork returns the network validation result for a context.
func ValidateContextNetwork(contextName string, reader domain.StatusReader) (map[string]any, error) {
	return reader.ValidateContextNetwork(contextName)
}
