// Package app is the composition root for command-level workflows. It resolves
// one project root and constructs the config, story artifacts, LLM client and
// agents from that root, so a command does not have to repeat the same
// load-and-wire sequence (and cannot accidentally mix two projects).
package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/store"
)

// Session is an explicit project view plus the dependencies derived from it.
// Setup and Outline are loaded lazily so commands that only need novel.json do
// not fail early on a missing later-stage artifact.
type Session struct {
	Root    *store.Root
	Config  *models.ProjectConfig
	Setup   *models.StorySetup
	Outline *models.Outline

	LLMConfig *llm.Config
	Client    llm.Client
}

// Open resolves the project a command was started for and loads novel.json.
func Open(cmd *cobra.Command) (*Session, error) {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return nil, err
	}
	return OpenAt(book.Root)
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
	return &Session{Root: root, Config: config}, nil
}

// Book returns the project facade for callers that still use project.Book.
func (s *Session) Book() *project.Book {
	if s == nil {
		return nil
	}
	return project.Open(s.Root)
}

// RequireSetup loads and caches the story setup.
func (s *Session) RequireSetup() (*models.StorySetup, error) {
	if s == nil || s.Root == nil {
		return nil, fmt.Errorf("session has no project root")
	}
	if s.Setup != nil {
		return s.Setup, nil
	}
	setup, err := s.Root.LoadSetup()
	if err != nil {
		return nil, fmt.Errorf("failed to load story setup at %s: %w", s.Root.SetupJSON(), err)
	}
	s.Setup = setup
	return setup, nil
}

// RequireOutline loads and caches the outline.
func (s *Session) RequireOutline() (*models.Outline, error) {
	if s == nil || s.Root == nil {
		return nil, fmt.Errorf("session has no project root")
	}
	if s.Outline != nil {
		return s.Outline, nil
	}
	outline, err := s.Root.LoadOutline()
	if err != nil {
		return nil, fmt.Errorf("failed to load outline at %s: %w", s.Root.OutlineJSON(), err)
	}
	s.Outline = outline
	return outline, nil
}

func (s *Session) ensureLLM() error {
	if s == nil || s.Config == nil {
		return fmt.Errorf("session has no project config")
	}
	if s.Client != nil {
		return nil
	}
	cfg, err := s.LoadLLMConfig()
	if err != nil {
		return err
	}
	client := cfg.CreateClient(&s.Config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}
	s.Client = client
	return nil
}

// LoadLLMConfig loads the global LLM config without constructing a client.
// Commands that need a specific transport (for example the analysis commands
// that intentionally use the OpenAI-compatible HTTP client) can use this form.
func (s *Session) LoadLLMConfig() (*llm.Config, error) {
	if s == nil || s.Config == nil {
		return nil, fmt.Errorf("session has no project config")
	}
	if s.LLMConfig != nil {
		return s.LLMConfig, nil
	}
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load LLM config: %w", err)
	}
	s.LLMConfig = cfg
	return cfg, nil
}

// LoadLLM returns the config and client shared by every agent in this session.
// Commands that need a stage-specific constructor (for example a craft agent
// over a filtered outline) can use this instead of loading the global config
// themselves.
func (s *Session) LoadLLM() (*llm.Config, llm.Client, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, nil, err
	}
	return s.LLMConfig, s.Client, nil
}

// NewComposeAgent constructs a compose agent bound to this session's root.
func (s *Session) NewComposeAgent() (*agents.ComposeAgent, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, err
	}
	agent := agents.NewComposeAgent(s.Client, s.LLMConfig, &s.Config.LLM)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewSetupAgent constructs a setup agent.
func (s *Session) NewSetupAgent() (*agents.SetupAgent, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, err
	}
	agent := agents.NewSetupAgent(s.Client, s.LLMConfig, &s.Config.LLM)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewCraftAgent constructs a craft agent with the session's setup and outline.
func (s *Session) NewCraftAgent() (*agents.CraftAgent, error) {
	if err := s.ensureLLM(); err != nil {
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
	agent := agents.NewCraftAgent(s.Client, s.LLMConfig, &s.Config.LLM, setup, outline)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewDraftAgent constructs a legacy draft agent.
func (s *Session) NewDraftAgent() (*agents.DraftAgent, error) {
	if err := s.ensureLLM(); err != nil {
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
	agent := agents.NewDraftAgent(s.Client, s.LLMConfig, &s.Config.LLM, setup, outline)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewWriteAgent constructs the final-chapter agent.
func (s *Session) NewWriteAgent() (*agents.WriteAgent, error) {
	if err := s.ensureLLM(); err != nil {
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
	agent := agents.NewWriteAgent(s.Client, s.LLMConfig, &s.Config.LLM, setup, outline)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewRecapAgent constructs the recap extractor.
func (s *Session) NewRecapAgent() (*agents.RecapAgent, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, err
	}
	agent := agents.NewRecapAgent(s.Client, s.LLMConfig, &s.Config.LLM)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewChapterScorerAgent constructs the best-of scorer.
func (s *Session) NewChapterScorerAgent() (*agents.ChapterScorerAgent, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, err
	}
	agent := agents.NewChapterScorerAgent(s.Client, s.LLMConfig, &s.Config.LLM)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}

// NewTranslateAgent constructs a translation agent.
func (s *Session) NewTranslateAgent() (*agents.TranslateAgent, error) {
	if err := s.ensureLLM(); err != nil {
		return nil, err
	}
	agent := agents.NewTranslateAgent(s.Client, s.LLMConfig, &s.Config.LLM)
	agent.SetLanguage(s.Config.Language)
	agent.SetProjectRoot(s.Root)
	return agent, nil
}
