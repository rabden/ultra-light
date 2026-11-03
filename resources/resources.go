// Package resources holds Nanolight's pictures: the app's icon (from the
// Zig build's src/icon-256.png and icon.png).
package resources

import _ "embed"

// Icon256 is the app's icon, 256 pixels square: the panel's footer, the
// welcome, the desktop entry.
//
//go:embed icon-256.png
var Icon256 []byte

// Icon is the app's icon, 1024 pixels square.
//
//go:embed icon.png
var Icon []byte

// RaycastHost is the Node host that runs Raycast extensions
// (raycast/dist/host.js, copied here by go generate: Go embeds only files
// inside the module).
//
//go:generate cp ../raycast/dist/host.js raycast-host.js
//go:embed raycast-host.js
var RaycastHost []byte
