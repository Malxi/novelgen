// Package app is the composition root for command-level workflows. It resolves
// one project root and constructs the config, story artifacts, LLM client and
// agents from that root, so a command does not have to repeat the same
// load-and-wire sequence (and cannot accidentally mix two projects).
package app

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/store"
)

// Session is an explicit project view plus the dependencies derived from it.
// Its fields are private so every caller gets the same validated root/config
// pair. Setup and Outline are loaded lazily so commands that only need
// novel.json do not fail early on a missing later-stage artifact.
type Session struct {
	mu sync.Mutex

	root    *store.Root
	config  *models.ProjectConfig
	setup   *models.StorySetup
	outline *models.Outline

	llmConfig *llm.Config
	client    llm.Client
}

// Open resolves the project a command was started for and loads novel.json.
func Open(cmd *cobra.Command) (*Session, error) {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return nil, err
	}
	return OpenAt(book.Root())
}

// OpenAt builds a session over an already-resolved project root.
func OpenAt(root *store.Root) (*Session, error) {
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	config, err := root.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load novel.json: %w", err)
	}
	return NewAt(root, config)
}

// NewAt builds a session from dependencies a caller already loaded. Most
// commands should use Open/OpenAt; this constructor exists for composition-root
// helpers that already resolved a config and must not manufacture Session
// values field by field.
func NewAt(root *store.Root, config *models.ProjectConfig) (*Session, error) {
	if root == nil || root.Dir() == "" {
		return nil, fmt.Errorf("project root is required")
	}
	if config == nil {
		return nil, fmt.Errorf("project config is required")
	}
	return &Session{root: root, config: config}, nil
}

// Root returns the explicit project root this session was built for.
func (s *Session) Root() *store.Root {
	if s == nil {
		return nil
	}
	return s.root
}

// Config returns the loaded novel.json config. The pointer is shared for the
// lifetime of the session; callers must not replace it with another project's
// config.
func (s *Session) Config() *models.ProjectConfig {
	if s == nil {
		return nil
	}
	return s.config
}

// Client returns the cached LLM client, if LoadLLM has already created one.
func (s *Session) Client() llm.Client {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// LLMConfig returns the cached global config, if it has already been loaded.
func (s *Session) LLMConfig() *llm.Config {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llmConfig
}

// Book returns the project facade for callers that still use project.Book.
func (s *Session) Book() *project.Book {
	if s == nil || s.root == nil {
		return nil
	}
	return project.Open(s.root)
}

// RequireSetup loads and caches the story setup.
func (s *Session) RequireSetup() (*models.StorySetup, error) {
	if s == nil || s.root == nil {
		return nil, fmt.Errorf("session has no project root")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setup != nil {
		return s.setup, nil
	}
	setup, err := s.root.LoadSetup()
	if err != nil {
		return nil, fmt.Errorf("failed to load story setup at %s: %w", s.root.SetupJSON(), err)
	}
	s.setup = setup
	return setup, nil
}

// RequireOutline loads and caches the outline.
func (s *Session) RequireOutline() (*models.Outline, error) {
	if s == nil || s.root == nil {
		return nil, fmt.Errorf("session has no project root")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outline != nil {
		return s.outline, nil
	}
	outline, err := s.root.LoadOutline()
	if err != nil {
		return nil, fmt.Errorf("failed to load outline at %s: %w", s.root.OutlineJSON(), err)
	}
	s.outline = outline
	return outline, nil
}

func (s *Session) ensureLLMLocked() error {
	if s == nil || s.config == nil {
		return fmt.Errorf("session has no project config")
	}
	if s.client != nil {
		return nil
	}
	cfg, err := s.loadLLMConfigLocked()
	if err != nil {
		return err
	}
	client := cfg.CreateClient(&s.config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	s.client = client
	return nil
}

// LoadLLMConfig loads the global LLM config without constructing a client.
// Commands that need a specific transport (for example the analysis commands
// that intentionally use the OpenAI-compatible HTTP client) can use this form.
func (s *Session) LoadLLMConfig() (*llm.Config, error) {
	if s == nil || s.config == nil {
		return nil, fmt.Errorf("session has no project config")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLLMConfigLocked()
}

func (s *Session) loadLLMConfigLocked() (*llm.Config, error) {
	if s.llmConfig != nil {
		return s.llmConfig, nil
	}
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}
	s.llmConfig = cfg
	return cfg, nil
}

// LoadLLM returns the config and client shared by every agent in this session.
// Commands that need a stage-specific constructor (for example a craft agent
// over a filtered outline) can use this instead of loading the global config
// themselves.
func (s *Session) LoadLLM() (*llm.Config, llm.Client, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLLMLocked(); err != nil {
		return nil, nil, err
	}
	return s.llmConfig, s.client, nil
}

// NewComposeAgent constructs a compose agent bound to this session's root.
func (s *Session) NewComposeAgent() (*agents.ComposeAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewComposeAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewSetupAgent constructs a setup agent.
func (s *Session) NewSetupAgent() (*agents.SetupAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewSetupAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewCraftAgent constructs a craft agent with the session's setup and outline.
func (s *Session) NewCraftAgent() (*agents.CraftAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	setup, err := s.RequireSetup()
	if err != nil {
		return nil, err
	}
	outline, err := s.RequireOutline()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewCraftAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewDraftAgent constructs a legacy draft agent.
func (s *Session) NewDraftAgent() (*agents.DraftAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	setup, err := s.RequireSetup()
	if err != nil {
		return nil, err
	}
	outline, err := s.RequireOutline()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewDraftAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewWriteAgent constructs the final-chapter agent.
func (s *Session) NewWriteAgent() (*agents.WriteAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	setup, err := s.RequireSetup()
	if err != nil {
		return nil, err
	}
	outline, err := s.RequireOutline()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewWriteAgent(client, cfg, &config.LLM, setup, outline)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewRecapAgent constructs the recap extractor.
func (s *Session) NewRecapAgent() (*agents.RecapAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewRecapAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewChapterScorerAgent constructs the best-of scorer.
func (s *Session) NewChapterScorerAgent() (*agents.ChapterScorerAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewChapterScorerAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}

// NewTranslateAgent constructs a translation agent.
func (s *Session) NewTranslateAgent() (*agents.TranslateAgent, error) {
	cfg, client, err := s.LoadLLM()
	if err != nil {
		return nil, err
	}
	config := s.Config()
	agent := agents.NewTranslateAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)
	agent.SetProjectRoot(s.Root())
	return agent, nil
}
