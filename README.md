# ultra-light

Nanolight for Linux: a launcher panel for apps, math, Google and Raycast
extensions, written in Go on [mygo](https://mygo.egoist.dev) (native UI,
no web page in the panel).

```sh
go test ./...
go tool mygo build      # dist/linux-amd64: executable, .deb, tarball, PKGBUILD, install.sh
go run . --toggle       # or run a build; --toggle shows or hides a running copy
```

Needs Go 1.27.3 (fetched by `GOTOOLCHAIN=auto`), GTK 3 and WebKitGTK at run
time; Raycast extensions need Node.js 20 or later. The Raycast host is in
`raycast/` (`npm ci && npm run build`, then `go generate ./resources`).
