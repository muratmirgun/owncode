package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manifest records exactly which source and revision supplied an installation.
type Manifest struct {
	Source   string `json:"source"`
	Subdir   string `json:"subdir"`
	Revision string `json:"revision"`
	Digest   string `json:"digest"`
}

// Proposal holds bounded files for inspection before an installation changes disk.
type Proposal struct {
	Entry    Entry
	Manifest Manifest
	Files    map[string][]byte
}

// Prepare fetches a source without executing its content. Sources use owner/repo#skill/path.
func Prepare(ctx context.Context, source, name string) (Proposal, error) {
	proposal := Proposal{Files: map[string][]byte{}}
	parts := strings.SplitN(strings.TrimSpace(source), "#", 2)
	source = parts[0]
	subdir := "."
	if len(parts) == 2 {
		subdir = parts[1]
	}
	if !filepath.IsLocal(subdir) {
		return proposal, fmt.Errorf("skill subdirectory must be relative")
	}
	directory := source
	revision := "local"
	if strings.HasPrefix(source, "https://") || (!filepath.IsAbs(source) && !strings.HasPrefix(source, ".")) {
		if !strings.HasPrefix(source, "https://") {
			source = "https://github.com/" + source
		}
		u, err := url.Parse(source)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" {
			return proposal, fmt.Errorf("use an HTTPS repository URL without credentials")
		}
		temp, err := os.MkdirTemp("", "owncode-skill-*")
		if err != nil {
			return proposal, err
		}
		defer func() { _ = os.RemoveAll(temp) }() // Best-effort cleanup; preserve the primary result.
		directory = filepath.Join(temp, "repo")
		ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-c", "core.hooksPath=/dev/null", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "clone", "--depth=1", "--no-recurse-submodules", "--", source, directory)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + temp, "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
		cmd.WaitDelay = 100 * time.Millisecond
		if err := cmd.Run(); err != nil {
			return proposal, fmt.Errorf("fetch skill repository: %w", err)
		}
		out, err := exec.CommandContext(ctx, "git", "-C", directory, "rev-parse", "HEAD").Output()
		if err != nil {
			return proposal, err
		}
		revision = strings.TrimSpace(string(out))
	}
	directory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return proposal, err
	}
	if subdir == "." {
		matches := []string{}
		visited := 0
		err = filepath.WalkDir(directory, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			visited++
			if visited > 20000 {
				return fmt.Errorf("repository exceeds skill discovery limit")
			}
			if d.IsDir() && path != directory && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.Name() != "SKILL.md" || !d.Type().IsRegular() {
				return nil
			}
			rel, _ := filepath.Rel(directory, path)
			data, err := ReadFile(directory, rel)
			if err != nil {
				return err
			}
			entry, _, err := Parse(data)
			if err != nil {
				return nil
			}
			if name == "" || entry.Name == name {
				matches = append(matches, filepath.Dir(rel))
			}
			return nil
		})
		if err != nil {
			return proposal, err
		}
		if len(matches) != 1 {
			return proposal, fmt.Errorf("found %d matching skills; specify repository#path/to/skill", len(matches))
		}
		subdir = matches[0]
	}
	// os.Root also rejects a subdirectory symlink escaping the fetched repository.
	data, err := ReadFile(directory, filepath.Join(subdir, "SKILL.md"))
	if err != nil {
		return proposal, err
	}
	entry, _, err := Parse(data)
	if err != nil {
		return proposal, err
	}
	if name != "" && entry.Name != name {
		return proposal, fmt.Errorf("source skill name does not match selection")
	}
	skillDir := filepath.Join(directory, subdir)
	resolved, err := filepath.EvalSymlinks(skillDir)
	if err != nil {
		return proposal, err
	}
	rel, err := filepath.Rel(directory, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return proposal, fmt.Errorf("skill escapes repository")
	}
	total := 0
	err = filepath.WalkDir(resolved, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if path != resolved && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("skill contains a link or special file")
		}
		file, _ := filepath.Rel(resolved, path)
		if file == ".owncode-install.json" || file == ".disabled" {
			return nil
		}
		data, err := ReadFile(resolved, file)
		if err != nil {
			return err
		}
		total += len(data)
		if total > 4<<20 || len(proposal.Files) >= 128 {
			return fmt.Errorf("skill exceeds installation size limit")
		}
		proposal.Files[file] = data
		return nil
	})
	if err != nil {
		return proposal, err
	}
	proposal.Entry = entry
	proposal.Manifest = Manifest{Source: source, Subdir: filepath.ToSlash(subdir), Revision: revision, Digest: digest(proposal.Files)}
	return proposal, nil
}

func digest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		_, _ = fmt.Fprintf(hash, "%d:%s:%d:", len(name), name, len(files[name]))
		hash.Write(files[name])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// Installed reads a managed installation record. Unmanaged skills have no record.
func Installed(entry Entry) (Manifest, error) {
	var manifest Manifest
	data, err := ReadFile(filepath.Dir(entry.Path), ".owncode-install.json")
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}

// Apply activates an inspected proposal. expected prevents overwriting unreviewed changes.
func Apply(root string, proposal Proposal, expected string) error {
	if !validName.MatchString(proposal.Entry.Name) || len(proposal.Entry.Name) > 64 {
		return fmt.Errorf("invalid installation name")
	}
	if digest(proposal.Files) != proposal.Manifest.Digest {
		return fmt.Errorf("proposal content changed")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	lock := filepath.Join(root, ".install-lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return fmt.Errorf("another skill change is active: %w", err)
	}
	defer func() { _ = os.Remove(lock) }() // Best-effort cleanup; preserve the primary result.
	target := filepath.Join(root, proposal.Entry.Name)
	if expected != "" {
		if err := checkInstalled(target, expected); err != nil {
			return err
		}
	} else if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return fmt.Errorf("skill already exists; use update")
	}
	temp, err := os.MkdirTemp(root, ".stage-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }() // Best-effort cleanup; preserve the primary result.
	for name, data := range proposal.Files {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("invalid installation path")
		}
		path := filepath.Join(temp, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(proposal.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temp, ".owncode-install.json"), data, 0600); err != nil {
		return err
	}
	backup := filepath.Join(root, ".rollback-"+proposal.Entry.Name)
	if expected != "" {
		if _, err := os.Lstat(backup); !os.IsNotExist(err) {
			old, readErr := Installed(Entry{Path: filepath.Join(backup, "SKILL.md")})
			if readErr != nil {
				return fmt.Errorf("cannot verify previous backup: %w", readErr)
			}
			if err := checkInstalled(backup, old.Digest); err != nil {
				return err
			}
			if err := os.RemoveAll(backup); err != nil {
				return err
			}
		}
		if _, err := os.Stat(filepath.Join(target, ".disabled")); err == nil {
			if err := os.WriteFile(filepath.Join(temp, ".disabled"), []byte("disabled\n"), 0600); err != nil {
				return err
			}
		}
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(temp, target); err != nil {
		if expected != "" {
			_ = os.Rename(backup, target)
		}
		return err
	}
	return nil
}

func checkInstalled(target, expected string) error {
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("installation is not a directory")
	}
	files := map[string][]byte{}
	err = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("installation contains a link")
		}
		rel, _ := filepath.Rel(target, path)
		if rel == ".owncode-install.json" || rel == ".disabled" {
			return nil
		}
		data, err := ReadFile(target, rel)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	if err != nil {
		return err
	}
	if digest(files) != expected {
		return fmt.Errorf("installed files changed locally; preserve them before updating or removing")
	}
	return nil
}

// Remove deletes only an unchanged managed installation.
func Remove(entry Entry) error {
	manifest, err := Installed(entry)
	if err != nil {
		return fmt.Errorf("only managed skills can be removed: %w", err)
	}
	target := filepath.Dir(entry.Path)
	lock := filepath.Join(filepath.Dir(target), ".install-lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return err
	}
	defer func() { _ = os.Remove(lock) }() // Best-effort cleanup; preserve the primary result.
	if err := checkInstalled(target, manifest.Digest); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

// RollbackProposal previews the last managed revision without changing the current installation.
func RollbackProposal(ctx context.Context, entry Entry) (Proposal, error) {
	backup := filepath.Join(filepath.Dir(filepath.Dir(entry.Path)), ".rollback-"+entry.Name)
	manifest, err := Installed(Entry{Path: filepath.Join(backup, "SKILL.md")})
	if err != nil {
		return Proposal{}, fmt.Errorf("no rollback revision: %w", err)
	}
	if err := checkInstalled(backup, manifest.Digest); err != nil {
		return Proposal{}, err
	}
	proposal, err := Prepare(ctx, backup, entry.Name)
	if err != nil {
		return Proposal{}, err
	}
	if proposal.Manifest.Digest != manifest.Digest {
		return Proposal{}, fmt.Errorf("rollback revision changed")
	}
	proposal.Manifest = manifest
	return proposal, nil
}
