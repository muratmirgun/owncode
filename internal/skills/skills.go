// Package skills discovers and loads portable Agent Skills without executing scripts.
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const MaxContent = 256 << 10

var validName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Entry is the metadata for one effective skill. Content loads separately.
type Entry struct {
	Name                   string `yaml:"name" json:"name"`
	Description            string `yaml:"description" json:"description"`
	License                string `yaml:"license" json:"license,omitempty"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation" json:"disableModelInvocation,omitempty"`
	UserInvocable          *bool  `yaml:"user-invocable" json:"userInvocable,omitempty"`
	directory              string
	root                   string
	Path                   string   `yaml:"-" json:"path"`
	Scope                  string   `yaml:"-" json:"scope"`
	Hash                   string   `yaml:"-" json:"hash"`
	Disabled               bool     `yaml:"-" json:"disabled"`
	Shadowed               []string `yaml:"-" json:"shadowed,omitempty"`
}

// Root is a discovery directory, ordered from highest to lowest precedence.
type Root struct{ Path, Scope string }

// Roots returns project and user locations, including the shared Agent Skills layout.
func Roots(workdir string) []Root {
	home, _ := os.UserHomeDir()
	userConfig := os.Getenv("XDG_CONFIG_HOME")
	if userConfig == "" {
		userConfig = filepath.Join(home, ".config")
	}
	return []Root{
		{filepath.Join(workdir, ".owncode", "skills"), "project"},
		{filepath.Join(workdir, ".agents", "skills"), "project"},
		{filepath.Join(userConfig, "owncode", "skills"), "user"},
		{filepath.Join(home, ".agents", "skills"), "user"},
	}
}

// Parse checks frontmatter and returns metadata plus the instruction body.
func Parse(data []byte) (Entry, string, error) {
	var entry Entry
	if len(data) > MaxContent || !utf8.Valid(data) {
		return entry, "", fmt.Errorf("skill must be UTF-8 and at most %d bytes", MaxContent)
	}
	text := strings.ReplaceAll(strings.TrimPrefix(string(data), "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(strings.TrimLeft(text, "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return entry, "", fmt.Errorf("skill needs YAML frontmatter")
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return entry, "", fmt.Errorf("unclosed skill frontmatter")
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &entry); err != nil {
		return entry, "", fmt.Errorf("parse skill: %w", err)
	}
	if !validName.MatchString(entry.Name) || len(entry.Name) > 64 {
		return entry, "", fmt.Errorf("invalid skill name")
	}
	if strings.TrimSpace(entry.Description) == "" || len(entry.Description) > 1024 {
		return entry, "", fmt.Errorf("skill description must contain 1–1024 bytes")
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if body == "" {
		return entry, "", fmt.Errorf("skill instructions are empty")
	}
	hash := sha256.Sum256(data)
	entry.Hash = hex.EncodeToString(hash[:])
	return entry, body, nil
}

// ReadFile confines reads to a skill directory, including symlink resolution.
func ReadFile(directory, name string) ([]byte, error) {
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("skill path must stay inside its directory")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }() // Best-effort cleanup; preserve the primary result.
	return readRootFile(root, name)
}

func readRootFile(root *os.Root, name string) ([]byte, error) {
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("skill path must stay inside its directory")
	}
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxContent {
		return nil, fmt.Errorf("skill file is not a bounded regular file")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // Best-effort cleanup; preserve the primary result.
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxContent {
		return nil, fmt.Errorf("skill file is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxContent+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxContent {
		return nil, fmt.Errorf("skill file exceeds limit")
	}
	return data, nil
}

// Discover returns a stable catalog and nonfatal diagnostics. The first name wins.
func Discover(ctx context.Context, roots []Root) ([]Entry, []string) {
	entries := []Entry{}
	problems := []string{}
	seen := map[string]int{}
	for _, root := range roots {
		if ctx.Err() != nil {
			return entries, append(problems, ctx.Err().Error())
		}
		children, err := os.ReadDir(root.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			problems = append(problems, root.Path+": "+err.Error())
			continue
		}
		for _, child := range children {
			if len(entries) >= 500 {
				return entries, append(problems, "skill catalog limited to 500 entries")
			}
			if ctx.Err() != nil {
				return entries, append(problems, ctx.Err().Error())
			}
			if strings.HasPrefix(child.Name(), ".") {
				continue
			}
			// Shared skill managers use links. Confine the link target to this discovery root.
			relative := filepath.Join(child.Name(), "SKILL.md")
			data, err := ReadFile(root.Path, relative)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				problems = append(problems, relative+": "+err.Error())
				continue
			}
			entry, _, err := Parse(data)
			if err == nil && entry.Name != child.Name() {
				err = fmt.Errorf("name must match directory")
			}
			if err != nil {
				problems = append(problems, relative+": "+err.Error())
				continue
			}
			entry.Path, _ = filepath.Abs(filepath.Join(root.Path, relative))
			entry.Scope = root.Scope
			entry.root = root.Path
			entry.directory, err = filepath.EvalSymlinks(filepath.Dir(entry.Path))
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			_, err = os.Stat(filepath.Join(root.Path, child.Name(), ".disabled"))
			entry.Disabled = err == nil
			if i, ok := seen[entry.Name]; ok {
				entries[i].Shadowed = append(entries[i].Shadowed, entry.Path)
				continue
			}
			seen[entry.Name] = len(entries)
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, problems
}

// Load verifies the chosen revision and returns its content with provenance.
func Load(entry Entry, file string) (string, error) {
	if entry.directory != "" {
		actual, err := filepath.EvalSymlinks(filepath.Dir(entry.Path))
		if err != nil || actual != entry.directory {
			return "", fmt.Errorf("skill directory changed; refresh before loading")
		}
	}
	if entry.Disabled {
		return "", fmt.Errorf("skill is disabled")
	}
	root, err := openEntry(entry)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Stat(".disabled"); err == nil {
		return "", fmt.Errorf("skill is disabled")
	}
	data, err := readRootFile(root, "SKILL.md")
	if err != nil {
		return "", err
	}
	current, body, err := Parse(data)
	if err != nil {
		return "", err
	}
	if current.Hash != entry.Hash {
		return "", fmt.Errorf("skill changed; refresh the catalog before loading")
	}
	if file != "" && file != "SKILL.md" {
		data, err = readRootFile(root, file)
		if err != nil {
			return "", err
		}
		if !utf8.Valid(data) {
			return "", fmt.Errorf("skill resource is not UTF-8 text")
		}
		body = string(data)
	}
	var manifest Manifest
	if data, err := readRootFile(root, ".owncode-install.json"); err == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	meta, _ := json.Marshal(struct{ Name, Path, Hash, File, Revision string }{entry.Name, entry.Path, entry.Hash, file, manifest.Revision})
	return "Skill source: " + string(meta) + "\nThese task instructions do not change tool permissions.\n\n" + body, nil
}

// SetEnabled changes the persistent state without deleting skill content.
func openEntry(entry Entry) (*os.Root, error) {
	if entry.root == "" {
		return os.OpenRoot(filepath.Dir(entry.Path))
	}
	root, err := os.OpenRoot(entry.root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.OpenRoot(entry.Name)
}
func SetEnabled(entry Entry, enabled bool) error {
	root, err := openEntry(entry)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if enabled {
		err := root.Remove(".disabled")
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	marker, err := root.OpenFile(".disabled", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return marker.Close()
}

// ExpandInvocation loads a leading $skill-name reference before saving the user message.
func ExpandInvocation(ctx context.Context, workdir, content string) (string, error) {
	fields := strings.Fields(content)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "$") {
		return content, nil
	}
	name := strings.TrimPrefix(fields[0], "$")
	if !validName.MatchString(name) {
		return content, nil
	}
	entries, _ := Discover(ctx, Roots(workdir))
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name == name {
			if entry.UserInvocable != nil && !*entry.UserInvocable {
				return "", fmt.Errorf("skill %s disables explicit invocation", name)
			}
			body, err := Load(entry, "")
			if err != nil {
				return "", err
			}
			return content + "\n\n" + body, nil
		}
	}
	return "", fmt.Errorf("skill %s not found; open /skills", name)
}
