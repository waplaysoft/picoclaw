// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"github.com/sipeed/picoclaw/pkg/tools"
)

// subagentAgentConfig wraps AgentInstance to provide the interface needed by tools.SubagentManager.
// This avoids circular dependency between agent and tools packages.
type subagentAgentConfig struct {
	instance *AgentInstance
}

func (s *subagentAgentConfig) ID() string {
	return s.instance.ID
}

func (s *subagentAgentConfig) Name() string {
	return s.instance.Name
}

func (s *subagentAgentConfig) Model() string {
	return s.instance.Model
}

func (s *subagentAgentConfig) Fallbacks() []string {
	return s.instance.Fallbacks
}

func (s *subagentAgentConfig) Workspace() string {
	return s.instance.Workspace
}

func (s *subagentAgentConfig) MaxIterations() int {
	return s.instance.MaxIterations
}

func (s *subagentAgentConfig) MaxTokens() int {
	return s.instance.MaxTokens
}

func (s *subagentAgentConfig) Temperature() float64 {
	return s.instance.Temperature
}

func (s *subagentAgentConfig) ContextWindow() int {
	return s.instance.ContextWindow
}

func (s *subagentAgentConfig) Tools() *tools.ToolRegistry {
	return s.instance.Tools
}

func (s *subagentAgentConfig) SystemPrompt() string {
	if s.instance.ContextBuilder != nil {
		return s.instance.ContextBuilder.BuildSystemPromptWithCache()
	}
	return ""
}

func (s *subagentAgentConfig) SkillsFilter() []string {
	return s.instance.SkillsFilter
}

// subagentRegistry wraps AgentRegistry to provide the interface needed by tools.SubagentManager.
type subagentRegistry struct {
	registry *AgentRegistry
}

func (sr *subagentRegistry) GetAgent(agentID string) (*tools.AgentConfigForSubagent, bool) {
	instance, ok := sr.registry.GetAgent(agentID)
	if !ok {
		return nil, false
	}

	cfg := &subagentAgentConfig{instance: instance}
	return &tools.AgentConfigForSubagent{
		ID:            cfg.ID(),
		Name:          cfg.Name(),
		Model:         cfg.Model(),
		Fallbacks:     cfg.Fallbacks(),
		Workspace:     cfg.Workspace(),
		MaxIterations: cfg.MaxIterations(),
		MaxTokens:     cfg.MaxTokens(),
		Temperature:   cfg.Temperature(),
		ContextWindow: cfg.ContextWindow(),
		Tools:         cfg.Tools(),
		SystemPrompt:  cfg.SystemPrompt(),
		SkillsFilter:  cfg.SkillsFilter(),
	}, true
}

// newSubagentRegistry creates an adapter for AgentRegistry to be used by tools.SubagentManager.
func newSubagentRegistry(registry *AgentRegistry) tools.AgentRegistryForSubagent {
	return &subagentRegistry{registry: registry}
}
