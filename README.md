<p align="center">
  <img src="docs/assets/owncode.svg" width="720" alt="OwnCode" />
</p>

<p align="center"><strong>Your terminal. Your models. Your workflow.</strong></p>
<p align="center">An experimental coding agent with built-in Witch orchestration and configurable context compaction.</p>

<p align="center">
  <a href="https://github.com/muratmirgun/owncode/releases/latest"><img src="https://img.shields.io/github/v/release/muratmirgun/owncode?color=88c0d0" alt="Latest release" /></a>
  <a href="https://github.com/muratmirgun/owncode/actions/workflows/build.yml"><img src="https://github.com/muratmirgun/owncode/actions/workflows/build.yml/badge.svg" alt="Build status" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/muratmirgun/owncode?color=a3be8c" alt="MIT license" /></a>
</p>

<p align="center">
  <a href="#get-started">Install</a> ·
  <a href="#agents">Agents</a> ·
  <a href="#context-compaction">Compaction</a> ·
  <a href="docs/guide.md">Documentation</a> ·
  <a href="docs/roadmap.md">Roadmap</a>
</p>

---

OwnCode brings code editing, tool approvals, and parallel workers into one terminal interface.
Choose your model, switch between Build and Plan, or let Witch coordinate specialist workers.
Keep saved sessions and choose how OwnCode reduces their active context.

- **Work visibly.** Follow tool activity, inspect diffs, and approve requests above the message input.
- **Choose your models.** Connect a ChatGPT account, add a Claude API key, or configure a compatible endpoint.
- **Coordinate workers.** Open live subagent transcripts and send follow-ups to saved workers.
- **Control context.** Choose summary, Shake, Snapcompact, experimental Jev, or supported native compaction.
- **Extend your workflow.** Load skills, project instructions, MCP tools, and language servers.
- **Recover a turn.** Preview file changes before undoing or redoing a checkpoint.

## Get started

### Homebrew

```bash
brew install muratmirgun/tap/owncode
owncode
```

### Install script

```bash
curl -fsSL https://raw.githubusercontent.com/muratmirgun/owncode/main/install -o /tmp/owncode-install
bash /tmp/owncode-install
```

The installer verifies SHA-256 checksums and installs into `~/.owncode/bin`.
Follow its PATH instruction, then run `owncode`.
It leaves your shell configuration unchanged.

<details>
<summary>Choose a version or installation directory</summary>

```bash
bash /tmp/owncode-install --version v0.1.0
bash /tmp/owncode-install --install-dir "$HOME/.local/bin"
```

</details>

### Other installation methods

| Method | Instructions |
| --- | --- |
| Release archive | Download a binary from [Releases](https://github.com/muratmirgun/owncode/releases/latest). |
| Debian / RPM | Download the matching `.deb` or `.rpm` package from the release. |
| Go | Run `go install github.com/muratmirgun/owncode@latest` with Go 1.27.1 or later. |

Releases support **macOS and Linux**, with **arm64 and x86_64** binaries.
Git is required for repository operations. Windows binaries are not available.

### Connect a model

Start OwnCode inside your project:

```bash
cd your-project
owncode
```

1. Enter `/connect` to connect a ChatGPT account or add a Claude API key.
2. Enter `/models` to select a model.
3. Write a request and press `Enter`.

OwnCode opens without a provider. Sending requires a configured model.
For other endpoints, use the [custom provider example](docs/theykk.example.json).

## Agents

Press **Tab** in the message field to switch profiles while agents are idle.
Your draft stays intact.

| Profile | Purpose |
| --- | --- |
| **Build** | Edit code and run tools through the approval flow. |
| **Plan** | Explore the project with read-only tools. |
| **Witch** | Coordinate specialist workers from the main conversation. |
| **Custom** | Define your own prompt, tools, model, and reasoning level. |

### Built-in Witch

OwnCode includes a preset based on [Witch](https://github.com/muratmirgun/witch).
Select it with Tab or `/profiles`. The main conversation becomes the controller.

Open **Settings → Orchestration** to set models and reasoning levels for its eight roles.
Unset role models use the base chat model.
Implementation workers can edit assigned files; research and review workers remain read-only.

A batch runs up to three children concurrently. Workers share the checkout.
Open `/agents` to inspect activity, follow a transcript, or send a follow-up.
See the [Witch guide](docs/witch.md) for roles and boundaries.

## Context compaction

Choose **Settings → Context → Method**, then run **`/compact`**.
One command uses your saved method.

| Method | What it does |
| --- | --- |
| **Summary** | Creates a text summary with balanced, brief, or handoff modes. This is the default. |
| **Shake** | Archives attachments and eligible older, large tool results. |
| **Snapcompact** | Converts older text into image pages. Requires image support and remains experimental. |
| **Jev** | Uses [Compact Engine](https://github.com/muratmirgun/compact-engine) to score and shorten eligible tool text. |
| **Native** | Uses a provider's native compaction path when the endpoint supports it. |

**Jev is experimental and requires a separate API key.**
It preserves attachments and protected content, so you do not need to run Shake first.
If the engine cannot meet the reduction budget, OwnCode retains the original active history.

Automatic compaction defaults to 95% context usage. You can change the threshold in Settings.
A compatible chat API does not automatically support native compaction.
See [compaction setup and limits](docs/guide.md#context-compaction), including the Jev configuration example.

## Skills and project context

OwnCode reads ancestor `AGENTS.md` files and loads skill instructions on demand.
Use `/skills` to inspect, install, update, or roll back a skill.
Start a message with `$skill-name` to select one.

- Install skills from a local directory or a Git repository.
- Use project skills in `.owncode/skills/` or `.agents/skills/`.
- Add MCP servers for external tools.
- Connect language servers for diagnostics, definitions, references, and symbols.

Optional skills.sh search requires a valid catalog API token. Authenticated search has not been live-verified.
Git installation works without catalog access.
See [skills](docs/guide.md#skills-and-installation) and [server configuration](docs/guide.md#mcp-and-language-servers).

## Keyboard controls

| Key | Action |
| --- | --- |
| `Ctrl+P` / `Ctrl+K` / `F1` | Open the command palette |
| `Tab` | Switch agent profiles; complete an open suggestion |
| `Ctrl+O` / `F2` | Select a model |
| `Alt+R` / `F4` | Cycle supported reasoning levels |
| `Ctrl+S` / `F3` | Open saved sessions |
| `↑` / `↓` at the input boundary | Recall sent prompts or restore your draft |
| `Esc` | Close a dialog or cancel active work |

Use `/settings`, `/models`, `/profiles`, `/skills`, `/agents`, or `/help` if your terminal intercepts a shortcut.
Mac keyboards may require `Fn` for function keys.

## Configuration

Keep model definitions and keys in **`~/.owncode.json`**.
Project files can override global settings. Remove duplicate project entries when you want global values to apply.
Managed provider connections use a separate credentials file in the operating system's user configuration directory.

OwnCode uses its own configuration schema. It does not directly load current OpenCode configuration files or npm provider plugins.
See the [configuration guide](docs/guide.md#configuration) for search order, examples, and storage locations.

## Questions

### Is this the same project as OpenCode?

OwnCode is an independent fork of the archived [Go OpenCode project](https://github.com/opencode-ai/opencode), which continued as [Crush](https://github.com/charmbracelet/crush).
It is separate from the current [OpenCode](https://github.com/anomalyco/opencode) application.
The original MIT license and attribution remain intact.

### Can I resume a conversation?

Yes. OwnCode prints a resume command when you exit a saved chat:

```bash
owncode -s SESSION_ID
```

Run it from the same project directory and use the same data directory.
You can also select saved conversations through `/sessions`.

### Can I use OwnCode in scripts?

```bash
owncode -p "Explain this repository"
owncode -p "List the main packages" -f json
```

**Non-interactive mode automatically approves tool permissions for its session.**
See [scripting](docs/guide.md#scripting) before using it in automation.

### How stable is it?

OwnCode is experimental. Provider behavior depends on the model and endpoint.
Workers share files, recovery has limits, and some compaction methods require specific provider capabilities.
The [user guide](docs/guide.md) describes these boundaries.

## Development

```bash
git clone https://github.com/muratmirgun/owncode.git
cd owncode
go build -o bin/owncode .
go test ./...
go vet ./...
./bin/owncode
```

Use Go **1.27.1 or later** and Git.
See [development checks](docs/guide.md#development), the [roadmap](docs/roadmap.md), and [release instructions](docs/releasing.md).

Contributions are welcome. Keep changes focused and describe the checks you ran.
For interface changes, include a screenshot and terminal dimensions.

## License and credits

OwnCode uses the [MIT License](LICENSE).
Thanks to the original OpenCode contributors and the Charm community for the foundation.
See the [original credits](docs/guide.md#contributing-and-license) and [Witch license](docs/licenses/Witch-LICENSE).
