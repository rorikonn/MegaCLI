package megatool

import "charm.land/fantasy"

// Registry holds all registered MegaTools and provides lookup and
// batch-wrapping functionality.
type Registry struct {
	tools     []MegaTool
	onDisplay DisplayHandler
}

// NewRegistry creates a Registry with the given display handler.
func NewRegistry(onDisplay DisplayHandler) *Registry {
	return &Registry{onDisplay: onDisplay}
}

// Register adds a MegaTool to the registry.
func (r *Registry) Register(tool MegaTool) {
	r.tools = append(r.tools, tool)
}

// AsAgentTools returns all registered MegaTools as fantasy.AgentTool
// instances, with appropriate wrapping for non-Default modes.
func (r *Registry) AsAgentTools() []fantasy.AgentTool {
	result := make([]fantasy.AgentTool, len(r.tools))
	for i, t := range r.tools {
		if t.Mode() != ModeDefault {
			result[i] = &wrappedMegaTool{inner: t, onDisplay: r.onDisplay}
		} else {
			result[i] = t
		}
	}
	return result
}

// Get returns a registered MegaTool by name, or nil.
func (r *Registry) Get(name string) MegaTool {
	for _, t := range r.tools {
		if t.Info().Name == name {
			return t
		}
	}
	return nil
}
