// Package browserextension embeds the optional browser relay extension.
package browserextension

import "embed"

// Files contains the extension assets shipped with the OwnCode executable.
//
//go:embed manifest.json popup.html popup.js background.js
var Files embed.FS
