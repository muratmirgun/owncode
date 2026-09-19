package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muratmirgun/owncode/internal/config"
	"github.com/muratmirgun/owncode/internal/llm/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContextFromPaths(t *testing.T) {

	tmpDir := t.TempDir()
	_, err := config.Load(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.Get()
	cfg.WorkingDir = tmpDir
	cfg.ContextPaths = []string{
		"file.txt",
		"directory/",
	}
	testFiles := []string{
		"file.txt",
		"directory/file_a.txt",
		"directory/file_b.txt",
		"directory/file_c.txt",
	}

	createTestFiles(t, tmpDir, testFiles)

	context := getContextFromPaths()
	expectedContext := fmt.Sprintf("# From:%s/file.txt\nfile.txt: test content\n# From:%s/directory/file_a.txt\ndirectory/file_a.txt: test content\n# From:%s/directory/file_b.txt\ndirectory/file_b.txt: test content\n# From:%s/directory/file_c.txt\ndirectory/file_c.txt: test content", tmpDir, tmpDir, tmpDir, tmpDir)
	assert.Equal(t, expectedContext, context)
	for _, role := range []config.AgentName{config.AgentCoder, config.AgentTask} {
		prompt := GetAgentPrompt(role, models.ProviderOpenAI)
		assert.Contains(t, prompt, "The deepest applicable AGENTS.md takes precedence")
		assert.Contains(t, prompt, "file.txt: test content")
	}
	assert.NotContains(t, GetAgentPrompt(config.AgentTitle, models.ProviderOpenAI), "file.txt: test content")
}

func TestAgentInstructionsFollowDirectoryScope(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "project", "src")
	require.NoError(t, os.MkdirAll(work, 0755))
	files := map[string]string{
		"AGENTS.md":                    "parent rule",
		"project/AGENTS.md":            "project rule",
		"project/src/AGENTS.md":        "source rule",
		"project/src/nested/AGENTS.md": "nested rule",
		"project/other/AGENTS.md":      "unrelated rule",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0644))
	}
	context := processContextPaths(work, []string{"AGENTS.md"})
	assert.Contains(t, context, "parent rule")
	assert.Contains(t, context, "project rule")
	assert.Contains(t, context, "source rule")
	assert.NotContains(t, context, "nested rule")
	assert.NotContains(t, context, "unrelated rule")
	assert.Less(t, strings.Index(context, "parent rule"), strings.Index(context, "project rule"))
	assert.Less(t, strings.Index(context, "project rule"), strings.Index(context, "source rule"))
	assert.Equal(t, 1, strings.Count(context, "source rule"))
	// A provider rebuild sees changes instead of reusing a process-wide cache.
	require.NoError(t, os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("updated rule"), 0644))
	assert.Contains(t, processContextPaths(work, nil), "updated rule")
}

func TestAgentInstructionsLowercaseFallbackAndCustomPaths(t *testing.T) {
	work := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(work, "agents.md"), []byte("lowercase rule"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(work, "custom.md"), []byte("custom rule"), 0644))
	context := processContextPaths(work, []string{"custom.md"})
	assert.Contains(t, context, "lowercase rule")
	assert.Contains(t, context, "custom rule")
	require.NoError(t, os.WriteFile(filepath.Join(work, "AGENTS.md"), []byte("uppercase rule"), 0644))
	context = processContextPaths(work, nil)
	assert.Contains(t, context, "uppercase rule")
	assert.NotContains(t, context, "lowercase rule")
}

func createTestFiles(t *testing.T, tmpDir string, testFiles []string) {
	t.Helper()
	for _, path := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if path[len(path)-1] == '/' {
			err := os.MkdirAll(fullPath, 0755)
			require.NoError(t, err)
		} else {
			dir := filepath.Dir(fullPath)
			err := os.MkdirAll(dir, 0755)
			require.NoError(t, err)
			err = os.WriteFile(fullPath, []byte(path+": test content"), 0644)
			require.NoError(t, err)
		}
	}
}
