// Package cli holds process-wide CLI state shared by the command subpackages.
package cli

import "context"

// rootContext carries the process context (cancelled by SIGINT/SIGTERM) so
// helper functions that do not receive a *cobra.Command can still observe
// cancellation. It is set once, before any command runs.
var rootContext = context.Background()

// SetRootContext records the process context passed to ExecuteContext.
func SetRootContext(ctx context.Context) {
	rootContext = ctx
}

// CommandContext returns the process context for helpers called from RunE
// functions that do not thread a context through their signature.
func CommandContext() context.Context {
	if rootContext == nil {
		return context.Background()
	}
	return rootContext
}
