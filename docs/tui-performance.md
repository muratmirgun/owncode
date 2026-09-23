# TUI performance checks

The chat viewport draws visible rows and reuses an unchanged frame. Worker events use message indexes instead of scanning the full parent history. Tool snapshots share the existing 30 Hz conversation frame scheduler; keyboard and wheel input do not wait for that timer.

When you scroll into history, incoming content does not move the visible transcript. Click **New messages ↓** or press **End** to return to the live conversation.

Successful read and search tools start as one-line cards. Click a card to expand or collapse it. Expanded output shows up to 200 source lines (at most 32 KiB) and marks omitted output. Errors, commands, and edit diffs keep their existing open panels. These controls also work inside worker conversations.

## Reproduce the measurements

Run from the repository root:

```sh
GOSUMDB=sum.golang.org go test ./internal/tui/components/chat \
  -run '^$' -bench 'Benchmark(ToolHistoryActivity|VisibleTranscript|FiveWorkerInput)$' \
  -benchmem -benchtime=300ms -count=6 > /tmp/owncode-tui.txt

GOSUMDB=sum.golang.org go test ./internal/tui/components/chat \
  -run '^$' -bench BenchmarkToolHistoryActivity -benchtime=1s \
  -cpuprofile=/tmp/owncode-tui.cpu -o /tmp/owncode-tui.test

go tool pprof -top /tmp/owncode-tui.test /tmp/owncode-tui.cpu
```

Compare before and after files with `benchstat`. The harness uses local message fixtures. It does not call model providers.

## September 23, 2026 measurements

Apple M1 Pro, macOS arm64, Go 1.27.1. Six runs per version, compared against v0.4.1 with the same benchmark fixture.

| Scenario | Before | After | Change |
| --- | ---: | ---: | ---: |
| Activity state with 1,000 completed tools | 1.341 ms | 0.113 ms | −91.6% |
| Five worker snapshots, scroll, input, and view | 0.858 ms | 0.741 ms | −13.6% |
| Five-worker allocated bytes per iteration | 1,217 KiB | 976 KiB | −19.8% |
| Five-worker allocations per iteration | 6,221 | 3,554 | −42.9% |
| Five-worker local p95 | 1.490 ms | 1.353 ms | −9.2% |

The listed comparisons have p=0.002 across six runs. The unchanged viewport benchmark now hits its frame cache and allocates no new screen buffer. This measures cache reuse, not the cost of scrolling to new rows.

The CPU profile originally identified repeated call/result comparison in `hasToolsWithoutResponse`. The new implementation uses result IDs and direct content parts. Its remaining work includes map construction rather than quadratic comparison.

These are local processing measurements. They do not measure provider TPS, terminal painting, network delay, or end-to-end input latency. A live session can have different results.

## Regression checks

- A burst of 100 tool updates schedules one conversation frame and retains the latest result.
- New messages preserve a reader's scroll position; clicking the notice returns to live content.
- Expanded tool cards show details, and errors stay visible.
- Viewport cache invalidates on content changes, scroll, and resize.
- Existing five-worker background-render and scroll tests remain part of the race suite.
