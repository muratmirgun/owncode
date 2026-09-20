# OwnCode

**A terminal coding assistant with visible tool activity, parallel exploration, and configurable context compaction.**

OwnCode helps you inspect a project, change code, and continue saved conversations from your terminal.
It combines a Go application with a Bubble Tea v2 interface and local SQLite storage.

[Get started](#get-started) · [Configuration](#configuration) · [Context compaction](#context-compaction) · [Development](#development) · [Roadmap](docs/roadmap.md)

## Project status

OwnCode is an experimental, independent fork of the archived [Go OpenCode project](https://github.com/opencode-ai/opencode).
That project continued as [Crush](https://github.com/charmbracelet/crush).
OwnCode retains its MIT license and attribution. It is separate from the current [OpenCode](https://opencode.ai) application.

Use a source build while the project develops. Provider behavior depends on the endpoint and model.
Skills, profiles, recovery, and metadata hooks are available as experimental features.
Catalog search needs a valid skills.sh API token. Local and Git skill installation work without catalog access.

## What you can do

| Area | Current capabilities |
| --- | --- |
| Chat | Centered welcome screen, growing message input, prompt recall, and saved sessions |
| Navigation | Searchable command palette, slash suggestions, model search, favorites, and recent models |
| Models | ChatGPT account connection, Claude API connection, custom compatible endpoints, and supported reasoning controls |
| Tools | Read, search, edit, write, patch, shell commands, URL fetches, and external MCP tools |
| Changes | Boxed tool output, numbered diff previews, and inline approval choices |
| Subagents | Parallel implementation, exploration, and review with live transcripts |
| Context | Usage bar, estimated live TPS, response history, and five compaction methods |
| Project context | Ancestor `AGENTS.md`, Markdown commands, and skills loaded on demand |
| Skills | Project/user discovery, Git installation, previews, updates, rollback, and optional skills.sh search |
| Agent profiles | Build, read-only Plan, Witch orchestration, and custom profiles |
| Recovery | Turn checkpoints, file previews, undo/redo, and checks against later edits |
| Code intelligence | LSP diagnostics, definitions, references, and document/workspace symbols |
| Interaction | Inline questions, worker follow-ups, and saved worker IDs |
| Extensions | Version 1 process hooks for completion metadata |

Subagents support read-only research/review and writable implementation roles. A batch runs up to three children concurrently.
The parent waits for the batch. Workers keep saved IDs and accept follow-ups.
Writable workers and isolated worktrees remain outside this implementation.

## Get started

Install Go **1.27.1** or later and Git. Then build OwnCode:

```bash
git clone https://github.com/muratmirgun/owncode.git
cd owncode
go build -o bin/owncode .
./bin/owncode
```

The interface opens without a provider. OwnCode preserves your draft and blocks sending until you configure a model.

1. Enter `/connect` to connect a provider.
2. Choose a ChatGPT account or a Claude API key.
3. Enter `/models` to select an available model.
4. Write a request and press `Enter`.

For a custom endpoint, add its configuration to `~/.owncode.json` before starting OwnCode.
Use the [compatible provider example](docs/theykk.example.json) as a template.
Replace its key and machine-specific MCP path. Keep credentials outside the repository.

The executable stays at `bin/owncode`. Add that directory to your `PATH` to use `owncode` from other directories.

```bash
# Work in another project.
./bin/owncode -c /path/to/project

# Resume a saved session in that project.
./bin/owncode -c /path/to/project -s SESSION_ID

# Inspect supported flags.
./bin/owncode --help
```

When you exit a saved chat, OwnCode prints its title and a command to resume it.
Sessions belong to the configured data directory. Use the same project directory when resuming.

## Daily controls

| Control | Action |
| --- | --- |
| `Ctrl+P`, `Ctrl+K`, or `F1` | Open the searchable command palette |
| `/` | Show built-in command suggestions |
| `Ctrl+O` or `F2` | Open model search |
| `Ctrl+S` or `F3` | Open saved sessions |
| `Alt+R` or `F4` | Cycle the model's supported reasoning levels |
| `Ctrl+N` | Start a new session |
| `Enter` | Send the current message |
| `Ctrl+E` | Compose in an external editor |
| `↑` on the first input line | Recall an earlier sent prompt |
| `↓` on the last input line | Move forward and restore your draft |
| `Esc` | Close a dialog or cancel active work, depending on focus |
| `Ctrl+C` | Open the exit confirmation |

On Mac keyboards, you may need `Fn` for function keys.
Typed commands remain available when your terminal intercepts shortcuts.
Your terminal controls the font family and size.

Use `/settings`, `/connect`, `/models`, `/themes`, `/sessions`, `/new`, `/compact`, `/agents`, or `/help`.

In model search, `Ctrl+F` or `F6` toggles a favorite. `Ctrl+A` or `F5` opens provider connections.
Recalled prompts remain editable. Prompt recall does not restore attachments.

Permission requests appear above the message input:

| Choice | Key | Scope |
| --- | --- | --- |
| Allow | `a` or `1` | Run this request once |
| Allow for session | `s` or `2` | Allow matching requests during this session |
| Deny | `d` or `3` | Reject this request |

Arrow keys select a choice. `Enter` confirms it. Press `v` to expand the request preview.

## Configuration

Keep provider keys, model definitions, and Jev settings in **`~/.owncode.json`**.
OwnCode selects the first available global file from this search order:

1. `$HOME/.owncode.json`
2. `$XDG_CONFIG_HOME/owncode/.owncode.json`
3. `$HOME/.config/owncode/.owncode.json`

It then merges `.owncode.json` and `.owncode.local.json` from the working directory, in that order.
Project values can still override global values, including models and keys.
Remove duplicate project entries when you want the global values to apply.
The local override file is Git-ignored. Invalid JSON stops startup.

Model and compact settings save to the selected global file, or `~/.owncode.json` when none exists.
Managed provider connections use `owncode/connections.json` under the operating system's user configuration directory.
That directory can differ from `~/.config` on macOS.

OwnCode uses `providers`, `agents`, and `mcpServers` in JSON.
It does not load OpenCode's npm provider plugins or directly accept their configuration schema.

Custom provider models use IDs such as `theykk/qwen38`.
Define the endpoint under `providers.<name>.baseURL` and models under `providers.<name>.models`.
Each model declares `name`, `contextWindow`, and `maxTokens`.
The agent roles are `coder`, `task`, `summarizer`, and `title`.

The [example configuration](docs/theykk.example.json) shows sampling options, image support, and interleaved reasoning.
Restart OwnCode after manually editing configuration files.

Common provider environment variables include `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, and `GROQ_API_KEY`.
Inherited adapters also cover Azure, Bedrock, Vertex, OpenRouter, and Copilot. Their catalogs need endpoint-specific validation.
Use the available model list instead of assuming an older model name still works.

### Project instructions

OwnCode reads `AGENTS.md` from ancestor directories through the working directory.
It uses `agents.md` as a fallback when the uppercase file is absent.
More specific instructions take precedence within their directory.

Agents receive instructions to check nested instruction files before working in those directories.
Nested discovery is agent-driven; startup does not read every directory.
Restart OwnCode after instruction changes to refresh an existing agent.
Configured context files, including `OwnCode.md` and `OWNCODE.md`, remain supported.

### MCP and language servers

Add external tools and diagnostics through configuration:

```json
{
  "mcpServers": {
    "example": {
      "type": "stdio",
      "command": "/absolute/path/to/mcp-server",
      "args": []
    }
  },
  "lsp": {
    "go": {
      "command": "gopls",
      "disabled": false
    }
  }
}
```

Install each server separately. MCP also supports SSE connections with `url` and optional `headers`.
The agent receives LSP diagnostics. General LSP navigation and rename tools are not exposed yet.

### Custom commands

Store reusable prompts as Markdown files in these directories:

- `~/.config/owncode/commands/`, or the equivalent under `$XDG_CONFIG_HOME`
- `~/.owncode/commands/`
- `<data.directory>/commands/`, which defaults to `<project>/.owncode/commands/`

Commands appear in the palette with `user:` or `project:` prefixes.
Subdirectories become name segments. For example, `git/review.md` becomes `user:git:review`.
Use `$NAME` placeholders for named arguments; OwnCode asks for their values before sending the prompt.
These files are prompt templates, not executable plugins or discovered skills.

### Local data

By default, `.owncode/` contains session data and `owncode.db`.
Context archives also use the configured data directory.
OwnCode does not automatically import OpenCode settings or sessions.

## Context compaction

Choose **Settings → Context → Method**, then run **`/compact`**.
The command uses your saved method without opening another selector.

| Method | Behavior | Requirement |
| --- | --- | --- |
| `summary` | Summarize the active conversation for continued work | Configured summarizer model |
| `shake` | Archive attachments and eligible older, large tool results | Local archive storage |
| `snapcompact` | Render older text as image pages and keep recent messages | Model with image support; experimental |
| `jev` | Score eligible tool text and shorten selected results through Compact Engine | Separate Jev API key |
| `native` | Use the provider's native compaction path | Explicit support from the provider and endpoint |

Summary is the default method. Summary modes are `balanced`, `brief`, and `handoff`.
You can also set a focus, such as test failures or unfinished changes.
Automatic compaction defaults to enabled at 95% context usage. Settings offers 70%, 80%, 90%, and 95% triggers.
Compaction updates the active context in the existing session; the original transcript remains stored.

### Jev setup

Merge this object into your global configuration:

```json
{
  "compaction": {
    "method": "jev",
    "jev": {
      "apiKey": "YOUR_JEV_API_KEY"
    }
  }
}
```

OwnCode integrates [Compact Engine](https://github.com/muratmirgun/compact-engine) through its Go library, currently pinned to `v0.2.0`.
Jev evaluates whether eligible tool results need full retention.
OwnCode preserves attachments and other protected content; you do not need to run Shake first.
It does not summarize ordinary chat prose in place of tool results.
Eligible context goes to the Jev scoring service when you use this method.

If the engine reports `budget_unmet`, OwnCode retains the original active history.
A requested reduction is not guaranteed when protected content or scoring prevents it.
Use summary compaction when you need a different reduction strategy.

An OpenAI-compatible chat endpoint does not automatically support native compaction.
Its Responses support must match the native adapter. Enabling a config flag cannot add missing endpoint support.
Snapcompact can reject mixed history or a conversion that would increase estimated context size.
TPS values are estimates, not an independent provider benchmark.

## Skills and installation

OwnCode follows Crush's catalog-first pattern: metadata enters the tool description; full instructions load when selected.
The implementation is independent. It does not execute scripts while loading or installing a skill.

Discovery uses this precedence:

1. `<project>/.owncode/skills/<name>/SKILL.md`
2. `<project>/.agents/skills/<name>/SKILL.md`
3. `$XDG_CONFIG_HOME/owncode/skills/<name>/SKILL.md`, or `~/.config/owncode/skills/<name>/SKILL.md`
4. `~/.agents/skills/<name>/SKILL.md`

A minimal skill:

```markdown
---
name: review-go
description: Review Go changes for errors and missing checks.
---
Read the changed functions. Report concrete failures with file paths and evidence.
```

The name must match its directory. Names use lowercase letters, numbers, and single hyphens, up to 64 characters.
Optional frontmatter fields include `license`, `user-invocable: false`, and `disable-model-invocation: true`.
The first copy wins. The detail view lists shadowed paths. A disabled override does not reveal a lower-priority copy.

Enter `/skills` to manage skills:

| Control | Action |
| --- | --- |
| `Tab` | Switch between Installed and Discover |
| `Enter` | Inspect a skill or search the catalog |
| `F2` / `F3` | Prepare a project / user installation |
| `Ctrl+E` | Enable or disable the selected skill |
| `Ctrl+U` | Prepare an update |
| `Ctrl+B` | Preview the previous installed revision |
| `Ctrl+D` | Confirm removal of an unchanged managed skill |
| `Ctrl+A` | Prepare the selected catalog result for project installation |
| `Ctrl+R` | Refresh installed entries |

Sources accept `owner/repo#path/to/skill`, an HTTPS Git URL, or a local directory.
OwnCode resolves a commit, stages files, and shows the source, revision, files, and instructions before installation.
An update checks the current content hash and retains one rollback revision.
Local edits block updates and removal. Installation does not run repository scripts or Git hooks.

Start a message with `$review-go` to select a skill. Suggestions appear after `$`.
`Enter` completes the skill name; it does not send the message.
You can also inspect a skill and press `a` to insert its instructions into the draft.
The agent can use the `skill` tool to load instructions and confined supporting files.
Loaded instructions carry their source path, content hash, and installed revision in the transcript.

Limits: 500 discovered skills, 256 KiB per text file, and 128 files / 4 MiB per installation.
Escaping paths and installation symlinks are rejected. Existing tool permissions still apply.
Shared-directory symlinks must stay inside their discovery root.

Discover uses the documented skills.sh API. Set `OWNCODE_SKILLS_TOKEN` to a valid catalog API token if you have one.
This requires the API's supported authentication, not a model API key.
Search metadata stays cached in memory for fifteen minutes. Failed refreshes clearly mark cached results.
OwnCode does not scrape the site or deploy a catalog service. Authenticated catalog access has not been live-verified.
Git installation remains available when search fails.

## Agent profiles and worker follow-ups

Enter `/profiles`, or use **Settings → Model → Agent profile**.
Press **Tab** in the message field to cycle through Build, read-only Plan, Witch, and custom profiles.
Your draft stays intact.
Tab still completes command suggestions. Switching requires idle agents.
Build permits the standard coding tools. Plan restricts the actual tool list to read-only tools.
An unknown configured profile falls back to a restricted planning policy.

Use **Settings → Orchestration** to configure the built-in **Witch** preset.
Each of its eight roles has a model selector and supported reasoning choices.
Select Witch through `/profiles` or Tab. The main chat becomes the controller.
Unset role models use the base chat model. No paid model is selected automatically.

Implementation workers can edit declared files and use shell commands through the existing approval flow.
Research and review workers stay read-only. Plan cannot start writable workers.
Witch routing chooses security, research, complex integration, then routine implementation.
Workers share the checkout; they do not run in isolated worktrees.
See [Witch orchestration](docs/witch.md) for the role matrix and boundaries.

Define custom profiles in the global configuration:

```json
{
  "activeProfile": "review",
  "profiles": {
    "review": {
      "description": "Read-only review",
      "readOnly": true,
      "prompt": "Review changes. Report concrete defects with evidence.",
      "tools": ["glob", "grep", "ls", "view", "lsp", "diagnostics", "skill", "ask"]
    }
  }
}
```

A profile can also set `model` to a registered model ID and set `reasoning`.
Those values override the coding role while the profile is active. Other model roles retain their configuration.
Omit `tools` to use the profile's default policy. An empty list exposes no tools.
Profiles change only while the agent is idle.

The `agent` tool returns a `worker_id`. A later call with `worker_id` and `prompt` resumes that child's saved history.
Only workers from the same parent session can resume. Active workers reject duplicate runs.

In `/agents`, press `f` to write a follow-up:

- An active worker receives the text after its current turn finishes.
- An idle worker prepares a resume request in the main draft. Press `Enter` there to send it.
- `c` cancels the selected worker. The queue accepts up to eight follow-ups.

Worker cancellation still follows the parent request. Follow-ups do not add write tools to a worker.
The `ask` tool presents inline options with a free-text reply. Dismissing a question grants no permission.

## Undo and redo

Enter `/undo` to inspect the last turn checkpoint. Enter `/redo` to inspect the next undone turn.
Review the diff before pressing `Enter`. `Esc` leaves the files and conversation intact.

A checkpoint includes conversation visibility, context counters, and regular files visible to Git.
OwnCode checks affected files before restoration. Later edits or conversation changes block the operation.
Unrelated paths remain intact. Billed cost remains intact too.

Recovery requires a Git working tree. It excludes ignored files, symlinks, submodules, OwnCode state files, and external effects.
Changes from other processes during the turn can enter the checkpoint; inspect the preview before restoring.
A new turn invalidates redo history. Old sessions do not gain checkpoints retroactively.

Limits: ten retained turns per session, 20,000 files, 8 MiB per file, and 32 MiB per capture.
Large captures fail without blocking the coding turn. The log explains unavailable checkpoints.
Completed checkpoints store changed files only. Large diffs show a notice instead of rendering an unlimited preview.
An interrupted restore blocks further restoration until the files receive manual inspection.

## Code intelligence and extensions

The read-only `lsp` tool supports `definition`, `references`, `document_symbols`, and `workspace_symbols`.
Choose a server when several are ready. Input line and character values start at one; characters use UTF-16 units.
Results retain standard LSP positions, which start at zero. Calls have a fifteen-second timeout.
A live `gopls` definition request is covered by an opt-in integration check:

```bash
OWNCODE_TEST_LSP="$(command -v gopls)" go test ./internal/llm/tools -run TestLSPGoplsIntegration -v
```
Rename and code actions are not exposed through this tool.

Optional process hooks use a versioned JSON protocol. Configure them in the **global** config only:

```json
{
  "extensions": {
    "local-notice": {
      "enabled": true,
      "command": ["/absolute/path/to/owncode-hook"]
    }
  }
}
```

Enable only trusted executables. Hooks run as your OS user; this is not a sandbox.
OwnCode ignores project-level hook definitions. Each process receives one JSON object through stdin:

```json
{"version":1,"type":"turn.complete","session_id":"example","model":"example/model","outcome":"completed"}
```

Return one JSON object through stdout:

```json
{"version":1,"notice":"Local task finished"}
```

The protocol includes no prompts, responses, file contents, or model credentials.
It permits a log notice, not tool execution requests or context changes.
At most four enabled hooks run within a shared two-second budget after a primary turn.
Output is limited to 8 KiB; notices are limited to 512 bytes. Hooks receive a minimal environment.
Canceled turns skip hooks. Hook failures appear in logs and do not discard the model response.

## Scripting

```bash
owncode -p "Explain this repository"
owncode -p "List the main packages" -f json
owncode -p "Explain this function" -q
```

**Non-interactive mode automatically approves tool permissions for its session.**
Use it only with prompts and projects where that behavior is appropriate.
`-f` selects text or JSON output. `-q` hides the spinner. `-d` enables debug logging.
The `-s` session flag cannot combine with `-p`.

## Development

```bash
go build -o bin/owncode .
go test ./...
go vet ./...
go test -race ./internal/skills ./internal/recovery ./internal/question ./internal/extension ./internal/llm/agent ./internal/llm/tools ./internal/tui/...
```

Use the stream/scroll fixture to compare terminal performance:

```bash
go test ./internal/tui/components/chat -run '^$' -bench 'Benchmark(ConversationScroll|WorkerStreamScrollInput|ChildThinkingFrame)$' -benchmem
```

This measures local event handling and rendering. It does not measure provider latency or terminal transport.

| Directory | Responsibility |
| --- | --- |
| `cmd/` | CLI, startup, and exit summary |
| `internal/tui/` | Chat, dialogs, input, and rendering |
| `internal/llm/` | Providers, agent execution, tools, and compaction |
| `internal/config/` | Settings and model registration |
| `internal/auth/` | Managed provider credentials |
| `internal/session/`, `internal/message/`, `internal/db/` | Saved conversations and storage |
| `internal/lsp/` | Language server integration |
| `internal/skills/` | Discovery, loading, Git installation, and catalog adapter |
| `internal/recovery/` | Turn checkpoints and restoration |
| `internal/question/`, `internal/extension/` | User questions and metadata hooks |

The [first-run report](docs/first-run.md) records the initial fork state, not the current feature set.
The [roadmap](docs/roadmap.md) records the implementation scope, reference sources, and remaining work.

## Contributing and license

Keep changes focused. Add relevant checks and describe the behavior you verified.
For interface changes, include the terminal size and a screenshot when possible.

OwnCode uses the [MIT License](LICENSE).
Thanks to the original OpenCode contributors and the Charm community for the foundation.
The original project also credits [isaacphi](https://github.com/isaacphi/mcp-language-server) for LSP work and [adamdottv](https://github.com/adamdottv) for interface direction.
