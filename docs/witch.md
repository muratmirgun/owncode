# Witch orchestration

OwnCode includes an adapted Witch preset. It does not load OpenCode JavaScript plugins.
Choose `/profiles` → `witch`, or press Tab in the composer.
The main chat becomes the controller. Build, Plan, and custom profiles remain available.

## Settings

Open **Settings → Orchestration**. Each role has three rows:

- **Model** opens the model selector with configured providers.
- **Reasoning** opens the supported levels for that role's selected model.
- **Use chat model** clears both overrides.

Changes require idle agents and persist in the global configuration.
An unset model uses the base chat model, not another worker's model.
Changing a worker does not change the controller or other workers.
F4 / Alt+R changes the controller's reasoning when Witch is active.

| Role | Responsibility | Tools |
| --- | --- | --- |
| controller | Route work and own final acceptance | Main chat tools |
| witch-routine | Bounded implementation | File edits, approved shell and fetch |
| witch-complex | Integration across files | File edits, approved shell and fetch |
| witch-researcher | Collect evidence | Read-only tools |
| witch-security | Security-sensitive implementation | File edits; no shell or network tools |
| witch-task-reviewer | Check one task | Read-only tools |
| witch-re-reviewer | Check repairs | Read-only tools |
| witch-final-reviewer | Check the complete change | Read-only tools |

Example global configuration:

```json
{
  "activeProfile": "witch",
  "witch": {
    "lanes": {
      "controller": {"model": "your-controller-model-id", "reasoning": "medium"},
      "witch-routine": {"model": "your-worker-model-id", "reasoning": "high"}
    }
  }
}
```

Use model IDs and reasoning levels from your configured catalog.
Project configuration cannot override the global Witch settings.

## Dispatch and review

The controller calls `witch_route` before each work assignment.
The agent tool checks the same route fields when dispatching.
Security signals take precedence, followed by research, complex integration, and routine implementation.
Review roles do not require a work route.

Writable workers require exact `owned_paths`. File edit tools reject other paths.
Concurrent writable workers cannot reserve overlapping files.
Saved worker resumes preserve the original role, route, and owned paths.
Workers cannot delegate recursively.

The controller prompt requires task review, bounded repair, and fresh final review.
The two-repair limit and final verdict remain controller instructions, not an enforced workflow state machine.

Build can also dispatch the `implement` role through the agent tool.
Plan permits only `explore` and `review` workers.

## Execution boundaries

Workers share the current checkout. They do not have isolated worktrees or an operating-system sandbox.
File ownership guards cover file tools. Approved shell commands can access other paths.
The shell retains its existing approval rules and serial execution.
Only disjoint agent work runs concurrently, with the existing limit of three workers.
Approval prompts queue one at a time. Cancellation releases queued and active requests.
Research and review workers do not receive edit, shell, or fetch tools.
Installed skills can provide instructions but cannot grant tools.

## Attribution

The role design and workflow derive from [muratmirgun/witch](https://github.com/muratmirgun/witch).
Witch uses the Apache License 2.0. Its license is included in [Witch-LICENSE](licenses/Witch-LICENSE).
This adaptation uses original Go code and adapted controller instructions.
It does not copy Witch's vendored Superpowers templates.
