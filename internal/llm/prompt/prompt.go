package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/muratmirgun/owncode/internal/logging"
)

func GetAgentPrompt(agentName config.AgentName, provider models.ModelProvider) string {
	basePrompt := ""
	switch agentName {
	case config.AgentCoder:
		basePrompt = CoderPrompt(provider)
	case config.AgentTitle:
		basePrompt = TitlePrompt(provider)
	case config.AgentTask:
		basePrompt = TaskPrompt(provider)
	case config.AgentSummarizer:
		basePrompt = SummarizerPrompt(provider)
	default:
		basePrompt = "You are a helpful assistant"
	}

	if agentName == config.AgentCoder || agentName == config.AgentTask {
		// Add context from project-specific instruction files if they exist
		contextContent := getContextFromPaths()
		logging.Debug("Loaded project instructions", "bytes", len(contextContent))
		basePrompt += "\n\n" + agentInstructionsPolicy
		if contextContent != "" {
			return fmt.Sprintf("%s\n\n# Project-Specific Context\n Make sure to follow the instructions in the context below\n%s", basePrompt, contextContent)
		}
	}
	return basePrompt
}

const agentInstructionsPolicy = `# Repository instructions
AGENTS.md contains instructions for its directory and all descendant directories.
Ancestor instructions appear before deeper instructions. The deepest applicable AGENTS.md takes precedence when instructions conflict.
Use agents.md only when AGENTS.md is absent in the same directory.
Before reading or editing files in a nested directory, check for additional AGENTS.md files between the working directory and the target directory. Read them before doing the work.
Before working outside the working directory, check that target's ancestor instruction files too.
Do not apply instructions from unrelated directories. Explicit user instructions take precedence over repository instructions.`

func getContextFromPaths() string {
	cfg := config.Get()
	if cfg == nil {
		return ""
	}
	return processContextPaths(cfg.WorkingDir, cfg.ContextPaths)
}

func processContextPaths(workDir string, paths []string) string {
	var results []string
	var processed []os.FileInfo
	appendFile := func(path string) {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		for _, previous := range processed {
			if os.SameFile(previous, info) {
				return
			}
		}
		if result := processFile(path); result != "" {
			processed = append(processed, info)
			results = append(results, result)
		}
	}

	// Ordered loading makes directory precedence stable across runs.
	for _, path := range agentInstructionPaths(workDir) {
		appendFile(path)
	}
	for _, path := range paths {
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(workDir, path)
		}
		if !strings.HasSuffix(path, "/") {
			appendFile(fullPath)
			continue
		}
		_ = filepath.WalkDir(fullPath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() {
				appendFile(path)
			}
			return nil
		})
	}
	return strings.Join(results, "\n")
}

func agentInstructionPaths(workDir string) []string {
	dir, err := filepath.Abs(workDir)
	if err != nil {
		return nil
	}
	var directories []string
	for {
		directories = append(directories, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	slices.Reverse(directories)
	var paths []string
	for _, dir := range directories {
		for _, name := range []string{"AGENTS.md", "agents.md"} {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				paths = append(paths, path)
				break
			}
		}
	}
	return paths
}

func processFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return "# From:" + filePath + "\n" + string(content)
}
