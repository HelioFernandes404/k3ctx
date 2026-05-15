package usecases

import "github.com/systemframe/k3ctx/internal/application"

// ListContextStatus returns all active context status entries.
func ListContextStatus(reader application.StatusReader) ([]map[string]any, error) {
	return reader.ListContextStatus()
}

// ValidateContextNetwork returns the network validation result for a context.
func ValidateContextNetwork(contextName string, reader application.StatusReader) (map[string]any, error) {
	return reader.ValidateContextNetwork(contextName)
}
