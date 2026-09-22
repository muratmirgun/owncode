# Browser and computer control

OwnCode includes optional browser and desktop tools. Select them in **Settings → Automation**. Both start disabled.

All adapters are free. They need no paid browser service. Your model provider can still charge for requests.

## Browser choices

| Backend | Behavior | Requirements |
|---|---|---|
| Off | No browser actions | None |
| Embedded | Managed headless Chromium session per agent | Installed Chrome, Brave, or Chromium |
| Chrome | Visible isolated Chrome session per agent | Installed Chrome |
| Brave | Visible isolated Brave session per agent | Installed Brave |
| CDP | New agent tab in an explicitly configured browser | HTTP CDP discovery endpoint |
| Extension | New agent tabs or explicitly shared existing tabs | Bundled extension and local pairing |

Embedded means built-in browser automation, not an HTML pane inside the TUI. OwnCode embeds the Go CDP client. It does not download Chromium, Node.js, or Python. Managed sessions use temporary profiles. Personal tabs and logins stay separate. Close sessions to release resources; at most eight managed sessions can exist per backend.

This follows OMP's separation between managed, attached, and relay browser modes. See [OMP browser documentation](https://github.com/can1357/oh-my-pi/blob/main/docs/tools/browser.md). OwnCode does not currently provide OMP's cmux surface integration or arbitrary browser evaluation.

Enable a backend, then ask OwnCode to open a URL. The browser tool supports navigation, observations, CSS selectors, element references, form entry, key presses, scrolling, screenshots, and closing sessions. Re-observe after page changes. Generated element references can become stale.

## Browser actions

| Actions | Parameters |
|---|---|
| `open`, `navigate`, `new_tab` | `url` |
| `tabs`, `switch_tab`, `close_tab` | Use a `tab` ID from `tabs`; omission targets the active tab |
| `observe`, `screenshot` | Optional `tab` |
| `click`, `hover`, `double_click`, `right_click` | CSS `selector` or observed `element` |
| `type`, `select` | Target and `text`; select uses the option value |
| `drag` | Source target and destination `targetSelector` |
| `upload` | File input target and absolute `files` paths |
| `press` | `key` and optional `modifiers` |
| `scroll` | Horizontal `x` and vertical `y` pixel deltas |
| `back`, `forward`, `reload` | Optional `tab` |
| `wait` | Visible target, bounded by the action timeout |
| `dialog` | `accept` and optional prompt `text` |
| `close` | Release the agent's owned tabs |

Each session supports eight owned tabs. Other agents cannot select those tabs. Explicitly shared extension tabs remain shared and survive `close`. Extension pointer actions bring the target tab forward.

Uploads use regular files that exist on the browser host. A remote CDP browser needs the same paths on its host. Upload approval exposes those files to the selected page. Downloads still follow the browser's own behavior; this tool does not manage download jobs.

The adapters use [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/tot/Input/) for input. Native pointer control uses Apple's [Quartz Event Services](https://developer.apple.com/documentation/coregraphics/quartz-event-services).

## Existing Chrome or Brave tabs

Run:

```sh
owncode browser setup
```

1. Open `chrome://extensions` or `brave://extensions`.
2. Enable Developer mode.
3. Select **Load unpacked**.
4. Select `~/.owncode/browser-extension`.
5. Select **Extension** in OwnCode's Automation settings.
6. Ask OwnCode to run `browser` with the `status` action.
7. Open the extension popup.
8. Paste the contents of `~/.owncode/browser-pairing.json` into the pairing box.
9. Select **Connect**.

Keep the pairing token out of chat. The relay binds only to loopback. It requires the token for every action. Each OwnCode process writes fresh pairing data. Pair again after restarting OwnCode. Use one paired OwnCode process at a time.

For an existing tab, select **Share this tab** in the extension popup. Supply its displayed ID when you ask OwnCode to use it. Without a shared ID, OwnCode creates a separate tab per agent. Closing a shared session leaves the user's tab open.

The extension uses Chrome's debugger permission. Chrome can display a debugging banner during actions. Select **Disconnect** to stop new requests. Already completed actions cannot be undone by cancellation. Closing the extension or restarting its worker can leave created tabs open; close these manually.

This extension is bundled for local loading. It is not published in a browser store. No additional JavaScript runtime is required.

## Native macOS control

Select **macOS** under Computer. The adapter uses the system's JavaScript for Automation and Accessibility APIs. It needs no third-party package.

Supported actions:

- List running applications.
- Inspect accessible elements in an application.
- Activate a selected application.
- Click an element by its unique accessible name.
- Type text and send supported keys to the foreground application.
- Click by an observed element reference.
- Move the pointer and perform left, right, or double clicks.
- Drag between two points and scroll a pane.
- Send shortcuts with Command, Control, Option, or Shift.
- Capture the primary display.

Allow Accessibility and Automation for the terminal that runs OwnCode. Allow Screen Recording for screenshots. macOS controls these permissions. OwnCode cannot grant them automatically.

Pointer actions provide a fallback when an application lacks useful accessibility names. Coordinates use the primary display only. Windows and Linux desktop adapters are not included.

Use `screenshot` before pointer actions. Coordinates run from 0 to 1000 across the screenshot: `(0,0)` is the top-left corner. `(1000,1000)` is the bottom-right corner. This remains correct when the model receives a resized image. Use `activate` first. The adapter rejects pointer input when the requested app is not in front.

`drag` requires `x`, `y`, `endX`, and `endY`. `scroll` uses pixel deltas: positive `x` moves right; positive `y` moves down. Scrolling occurs at the pointer position. Use `move` first for a specific pane. Each delta must stay between -2000 and 2000.

`press` accepts named keys or a single character. Optional `modifiers` accepts `Meta`, `Control`, `Alt`, and `Shift`. `Meta` means Command on macOS. For example, `key: "a", modifiers: ["Meta"]` selects all text.

Accessibility references are temporary. Inspect again after a window changes. Input actions cannot bypass secure fields or macOS permission controls. The adapter attempts to release mouse buttons after a canceled pointer action.

Desktop actions share one cancellable queue across all workers. Each action has a 30-second timeout. Native actions can change application focus. Browser sessions use separate queues.

## Screenshots

OwnCode stores captures in `~/.owncode/automation/screenshots`. Screenshots can contain private content. The tool sends a reduced image to models that declare attachment support. Text-only models receive the artifact path and observations.

Only the last four results in the latest tool batch supply screenshots to the next model request. Full captures remain on disk. Tool results retain the reduced image for session replay. Shake compaction can archive and remove those images from active history. There is no automatic disk retention policy yet.

## Global configuration

Project configuration cannot enable automation. Global settings accept:

```json
{
  "automation": {
    "browser": "embedded",
    "computer": "off",
    "executable": "",
    "cdpURL": ""
  }
}
```

`executable` optionally selects an absolute browser executable path. `cdpURL` selects an HTTP discovery endpoint, such as `http://127.0.0.1:9222`. Configure the external browser yourself before selecting CDP. Close existing sessions before changing launch paths or endpoints.

Plan, read-only workers, and Witch security workers do not receive these action tools. Writable agents use OwnCode's existing permission flow. Session approval retains its existing meaning. Treat webpage and desktop text as untrusted data.

## Adapter extension point

`internal/automation.Backend` defines `Run` and `Close`. `Registry` dispatches actions without holding its mutex during I/O. A future crawler can implement this interface, add its capabilities, and expose a settings choice. No crawler service is included in this release.

## Validation

Run the unit and race tests:

```sh
go test -race ./internal/automation ./internal/llm/agent ./internal/llm/tools
node --test browser-extension/background.test.cjs
```

Run the installed-browser test:

```sh
OWNCODE_BROWSER_TEST=1 go test -race ./internal/automation -run TestBrowserIntegration
```

The browser test uses a local fixture and an isolated headless profile. It does not use a model or personal browser session.

The optional extension integration test uses Brave in a temporary headless profile:

```sh
OWNCODE_EXTENSION_TEST=1 go test -race ./internal/automation -run TestExtensionIntegration
```

Native macOS inspection has an opt-in check. It lists application names without clicking or typing:

```sh
OWNCODE_COMPUTER_TEST=1 go test ./internal/automation -run TestComputerReadOnlyIntegration
```

Native event construction tests create Quartz events without posting them. They check the native bridge without controlling personal apps. Real desktop delivery depends on local permissions and application behavior.
