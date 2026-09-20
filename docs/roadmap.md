# OwnCode roadmap

Witch update: OwnCode now includes a selectable controller, per-role model and reasoning settings,
and writable implementation workers. See [Witch orchestration](witch.md).
Isolated worktrees and an enforced review state machine remain future work.


Research date: **2026-09-20**. OwnCode baseline: `f6a5417`.

This document preserves the baseline audit and records the implementation below.
The comparison table describes the baseline, not the new feature set.
The comparison uses current public documentation, not a runtime benchmark of either product.
Here, OMP means [can1357/oh-my-pi](https://github.com/can1357/oh-my-pi).
“Skills marketplace” refers to [skills.sh](https://skills.sh).

## Implementation status

The first implementation covers every roadmap area with bounded interfaces.
It does not claim full parity with OpenCode or OMP.

| Area | Implemented | Deliberate boundary |
| --- | --- | --- |
| Skills | Metadata discovery, lazy tool loading, `$name` suggestions, provenance, disable controls | No automatic script execution |
| Installation | Local/HTTPS Git sources, commit records, preview, update, one rollback revision, removal checks | No upstream Node CLI bridge |
| Marketplace | Installed/Discover UI and authenticated API adapter with bounded memory cache | No hosted service; authenticated live search remains unverified |
| Profiles | Build, restricted Plan, custom prompts/models/reasoning/tool allowlists | Writable isolated workers remain future work |
| Recovery | Ten turn checkpoints, diff confirmation, undo/redo, later-edit and conversation checks | Whole-turn restore; no partial-file restore, ignored files, or external side effects |
| Worker control | Persistent IDs, same-parent resume, active follow-up queue, cancellation | Queued steering begins after the current worker turn |
| LSP | Ready-client registry, definitions, references, document/workspace symbols | Read-only requests only |
| Questions | Inline options, free text, cancellation, noninteractive errors | One question per tool call |
| Capabilities | Settings explanations and eligible compact-method cycling | Remote endpoints still decide actual support |
| Performance | Three-stream input/scroll regression fixture and benchmarks | Synthetic event workload; no provider-speed claim |
| Extensions | Global opt-in process protocol, `turn.complete` metadata, bounded log replies | No arbitrary UI plugins or context mutation |

The [README](../README.md) documents commands, configuration, limits, and controls.
Crush's [skill catalog](https://github.com/charmbracelet/crush/tree/8c541ead9a431aae901b31ab5c5ab662f49caabe/internal/skills)
informed metadata-first discovery, deterministic precedence, and on-demand loading.
OwnCode's implementation is independent and uses its existing Go tool interfaces.

Next extensions can add update diffs, audit metadata, selective recovery, isolated writable workers, and additional hook events.
Those features require separate designs and acceptance tests. They are not implied by the initial interfaces.

## Local validation

Validated on macOS arm64 with Go 1.27.1:

- Full `go test -race ./...` suite and `go vet ./...` pass.
- Changed-code lint passes with golangci-lint 2.12.2 built through the current Go toolchain.
- Whole-repository lint still reports baseline issues outside this change.
- A clean-home terminal run verified skill discovery and `$` completion.
- A local provider fixture verified inline questions and conversation undo/redo through the terminal.
- Startup created the recovery tables. Recovery tests cover file restoration, later edits, file modes, and live database exclusion.
- A real `gopls` process returned a definition through the new tool and registry.
- The synthetic worker/scroll/input benchmark measured about 0.24 ms per cycle on an Apple M1 Pro.

The performance figure measures local processing, not API or terminal transport latency.
Live authenticated skills.sh search remains unverified. Catalog tests use an HTTP fixture.

## Direction

Build on OwnCode's existing strengths: visible agent work, responsive terminal input, and explicit context controls.
The next useful addition is a portable skill system with an installation interface.
A catalog alone cannot make downloaded instructions work inside the agent.

Recommended sequence:

1. Add local skill discovery and loading.
2. Add reproducible installation from Git repositories.
3. Add catalog browsing and updates.
4. Add configurable agent profiles and stronger recovery tools.
5. Expand worker control and code intelligence.

## Baseline gap comparison

These are selected product gaps, not an exhaustive parity checklist.
Effort estimates describe relative scope, not delivery dates.

| Capability | Reference behavior | OwnCode today | Proposed next step | Effort |
| --- | --- | --- | --- | --- |
| Skills | OpenCode discovers `SKILL.md` and loads content on demand [1]. OMP documents skill discovery [2]. | Ancestor instructions and command templates; no skill loader | Metadata catalog, lazy loading, `/skills` | Medium |
| Extensions | OpenCode exposes plugin events [3]. OMP bundles extensions and related assets [4]. | MCP tools and internal commands; no public extension contract | Versioned process protocol after skills | Large |
| Agent profiles | OpenCode supports primary agents, subagents, prompts, models, and permissions [5]. | Fixed coder/task/title/summarizer roles; explore/review children | Named profiles with explicit tool policies | Medium |
| Recovery | OpenCode offers message undo/redo with file restoration [6]. | Saved sessions, resume, and prompt recall | Turn checkpoints, preview, selective restoration | Large |
| Worker control | OMP describes live steering, worker revival, and optional worktree isolation [7]. | Up to three read-only children; live view and cancellation | Persistent worker IDs and follow-up messages first | Large |
| Code intelligence | OMP exposes navigation, symbols, rename, and code actions [7]. | Diagnostics tool; underlying LSP client | Definitions, references, and symbols before write actions | Medium |
| Structured questions | OMP includes an `ask` tool [7]. | Tool approval choices; no general question tool | Typed options and free-text replies | Medium |
| Model capabilities | OwnCode needs a local improvement here | Native compact and reasoning depend on adapter metadata | Capability checks with clear Settings explanations | Medium |
| Performance checks | OwnCode needs a local improvement here | Rendering batches and cached history | Repeatable stream-plus-scroll latency tests | Medium |

The last two rows are recommendations from the OwnCode audit, not claims about competitor advantages.

## Original skills marketplace design

### First: load a local skill

Use the common `SKILL.md` format with YAML `name` and `description` metadata.
OpenCode already uses a small catalog and an explicit tool to load the full instructions [1].
Adopt that pattern to avoid adding every installed skill to every prompt.

Proposed discovery locations, in precedence order:

1. `<project>/.owncode/skills/<name>/SKILL.md`
2. `<project>/.agents/skills/<name>/SKILL.md`
3. `~/.config/owncode/skills/<name>/SKILL.md`, respecting `XDG_CONFIG_HOME`
4. `~/.agents/skills/<name>/SKILL.md`

These paths now form the implemented discovery order.
Project overrides should appear in the interface with their source path.
Do not silently import every installed product's private skill directory.

Proposed interface:

- `/skills` opens **Installed**, **Discover**, and **Updates** tabs.
- Installed entries show their source, scope, version, and enabled state.
- A detail view shows instructions and supporting files.
- `$skill-name` explicitly selects a skill in the composer.
- The agent can call a `skill` tool when the metadata matches its task.
- A transcript entry records each loaded skill and its pinned revision.

Skill instructions must not grant tools or permissions by themselves.
Loading a skill does not execute its scripts. Existing tool permissions still apply.
Resolve supporting files within the installed skill directory.
Reject path traversal, escaping symlinks, and oversized content.

Acceptance checks:

- A local skill appears without a network request.
- Duplicate names resolve consistently and show the override.
- Disabled skills cannot load through the tool.
- Only selected skill content enters active context.
- Transcript records remain readable after the skill changes.

### Second: install from a repository

Install a selected skill folder at a resolved Git commit.
Keep its source URL, directory, commit, and content hash in a lock file.
Stage files before activation so a failed download leaves the existing version intact.
Show changed files before an update. Keep rollback and removal local and predictable.
Do not run package lifecycle scripts during installation.

The upstream Skills CLI accepts repository sources and specific skill selection [8].
OwnCode can follow the same source model without adding Node.js to its core runtime.
A Go installer should start with GitHub URLs and local directories; other source types can follow.
Treat an official Skills CLI adapter as a separate integration task, not assumed compatibility.

### Third: connect skills.sh discovery

The documented API provides search, leaderboard lists, skill details, and audit results.
It uses `/api/v1/` and documents Vercel OIDC authentication.
Missing or invalid tokens return `401`; throttling returns `429` [9].
This creates a deployment decision for a local Go application.

| Option | Benefit | Constraint |
| --- | --- | --- |
| Local and Git sources | No catalog service required | No built-in global search |
| Optional Skills CLI bridge | Uses upstream discovery and installation behavior | Adds Node.js and requires a supported output contract |
| OwnCode catalog service | Can serve a simple terminal client | Adds hosting, authentication, caching, and service maintenance |
| Direct API integration | Small client adapter | Requires a supported authentication path for local users |

Start with local and Git sources. Validate catalog access before committing to a hosted service.
Do not scrape leaderboard HTML as the application contract.
A public browsing page does not prove that every API route permits anonymous use.
No authenticated API request or deployment was performed during this research.

Keep discovery behind an adapter. Cache catalog metadata and show when it is stale.
Perform downloads and search outside the terminal update loop, with cancellation and timeouts.
Catalog failure must not block chat or installed skills.

Search results should show author, description, source, install scope, and update status.
If audit information exists, show its source and date. Popularity is not a trust guarantee.
Installation should show the exact files and revision before activation.

### Suggested implementation boundaries

| Area | Proposed responsibility | Existing integration point |
| --- | --- | --- |
| `internal/skills/` | Metadata, discovery, validation, loading, and install records | New package |
| Skill tool | Load selected content with provenance | `internal/llm/agent/tools.go` |
| Agent context | Expose bounded skill descriptions | `internal/llm/agent/` |
| Skills dialog | Installed list, search, detail, and updates | `internal/tui/components/dialog/` |
| Commands | `/skills` and palette entry | `internal/tui/components/chat/slash.go`, `internal/tui/palette.go` |
| Settings | Scope, enabled sources, and credentials if needed | `internal/config/`, Settings dialog |
| Catalog adapter | Optional network discovery | Separate from local loading |

MCP remains the mechanism for external tools.
Skills provide task instructions. Extensions change application behavior.
Keep those contracts separate even if one interface lists all three.

## Follow-on work

### Agent profiles and planning

Add named profiles with a model, reasoning level, prompt, and allowed tool set.
Provide a read-only planning profile before adding writable subagents.
Settings must show which profile and permissions are active.
This is more useful than adding many provider-specific shortcuts.

### Recovery and safe continuation

Add a checkpoint for each completed turn before exposing undo.
Record the file baseline and preserve unrelated user edits during restoration.
Show the restoration diff before applying it.
Include attachment and tool-call consistency in conversation rollback tests.
Prompt recall alone does not restore the project state.

### Worker interaction

Add follow-up messages to existing children before expanding concurrency.
Keep worker input, cancellation, and result delivery separate from the UI render path.
Writable workers need isolated directories plus an explicit integration step.
Increasing the worker limit alone would not provide these guarantees.

### Context and provider quality

Explain unavailable methods in Settings before a compact request fails.
Report the active method, input estimate, protected estimate, and achieved reduction.
Keep native compaction distinct from summary fallback.
Add fixtures for text-only history, tool-heavy history, attachments, and provider changes.

### Release readiness

Measure input latency while several workers stream and the user scrolls history.
Add provider contract checks and repeatable compact evaluations.
Before release, resolve the existing full-suite test failures and publish verified installation steps.
A polished marketplace should not hide failures in the core coding loop.

## Sources

- [1: OpenCode skills](https://opencode.ai/docs/skills/)
- [2: OMP skills](https://github.com/can1357/oh-my-pi/blob/main/docs/skills.md)
- [3: OpenCode plugins](https://opencode.ai/docs/plugins/)
- [4: OMP extension authoring](https://github.com/can1357/oh-my-pi/blob/main/docs/skills/authoring-extensions.md)
- [5: OpenCode agents](https://opencode.ai/docs/agents/)
- [6: OpenCode TUI commands](https://opencode.ai/docs/tui/)
- [7: OMP repository](https://github.com/can1357/oh-my-pi)
- [8: Skills CLI source](https://github.com/vercel-labs/skills)
- [9: skills.sh API](https://skills.sh/docs/api)

OwnCode evidence: `internal/config/config.go`, `internal/llm/agent/tools.go`, `internal/llm/agent/agent-tool.go`,
`internal/llm/agent/parallel.go`, `internal/llm/agent/compact_methods.go`, and the current TUI command registry.
