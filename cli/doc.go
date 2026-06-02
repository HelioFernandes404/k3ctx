// Package cli wires Cobra commands to use cases through the ServiceContainer.
// Each command owns its flags and runX handler; output helpers live next to
// the command that uses them. CLI code never imports infrastructure
// directly — it speaks to use cases and ports only.
package cli
