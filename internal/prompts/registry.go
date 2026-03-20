package prompts

// PromptRegistrar is a function that registers prompts with a manager
type PromptRegistrar func(*PromptManager)

// promptRegistrars holds all registered prompt registrars
var promptRegistrars []PromptRegistrar

// RegisterPrompts registers a prompt registrar function
// Call this in your prompt file's init() function
func RegisterPrompts(registrar PromptRegistrar) {
	promptRegistrars = append(promptRegistrars, registrar)
}

// registerAllPrompts registers all prompts that have been added to the registry
// This is called by NewPromptManager after setting up default prompts
func (pm *PromptManager) registerAllPrompts() {
	for _, registrar := range promptRegistrars {
		registrar(pm)
	}
}
