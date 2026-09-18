# First run: 2026-09-19

Environment: macOS arm64, Go 1.26.4.

## Results

| Check | Result |
| --- | --- |
| `go build -o bin/owncode .` | Passed |
| `./bin/owncode --help` | Passed; displays the OwnCode command |
| `./bin/owncode --version` | Passed; prints `unknown` for a source build without version flags |
| `go vet ./...` | Passed |
| `gofmt -l cmd internal main.go` | Passed; no output |
| `goreleaser check` | Passed after updating deprecated fields |
| `bash -n install` | Passed |
| `go test -race -short ./...` | Failed in the inherited directory-listing test |
| `golangci-lint run ./...` | Reported 78 inherited issues |

## Launch (initial commit)

The first launch without provider credentials returned `agent coder not found`.
A second launch used a dummy Anthropic key to check terminal startup only.
It displayed the project initialization dialog with the `OwnCode.md` name.
After declining initialization, the main screen displayed OwnCode and the new repository URL.
The quit dialog closed the application successfully.
The application created `.owncode/owncode.db` and applied its database migrations.
No prompt was submitted. No model response was tested.
The dummy key is not a working credential and is not saved in project settings.

A real provider configuration is required for the next model request.
The inherited model catalog also needs validation against that provider.

## Existing failures

`TestLsTool_Run/handles_empty_path_parameter` panics with `config not loaded`.
The unchanged source copy fails at the same location.
The test calls `config.WorkingDirectory()` without loading configuration first.

The unchanged source also reports 78 lint issues:

- 40 unchecked errors.
- 2 ineffective assignments.
- 25 static analysis findings.
- 11 unused declarations.

These issues remain outside this initial rename.
They need separate fixes before a production release.

## Source and scope

The source came from a local copy of the archived Go OpenCode project.
That copy had no Git metadata, so this repository starts with a new history.
The original MIT license and copyright notice remain intact.
The README links to the original project.

OwnCode now uses its own module path, command, settings, data paths, theme, and release assets.
The release configuration no longer targets the original Homebrew or AUR repositories.
Compact Engine integration remains future work.

## Update: startup without a model

OwnCode now opens without provider credentials.
The interface displays `No model configured` and keeps model actions disabled.
Enter preserves the draft and displays a setup notice.
No session or message is created for a blocked send.
Project initialization does not appear automatically until a model is available.
The empty model menu handles navigation and Enter without a panic.
Non-interactive prompts return a setup error before creating a session.
Configure a provider and restart OwnCode to enable model requests.

Validation:

- `go run main.go` opened the interface without a key.
- An Enter attempt preserved the draft; the database still contained zero sessions and zero messages.
- `go test -race ./internal/config ./internal/llm/agent ./internal/tui/...` passed.
- `go vet ./...` and the build passed.
- The linter still reported the same 78 inherited issues.
