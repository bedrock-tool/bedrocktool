iOS integration notes

1) Generate the iOS framework on macOS:
   - Install gomobile: `go install golang.org/x/mobile/cmd/gomobile@latest`
   - Run: `gomobile init` (first time only)
   - Generate framework: `scripts/gomobile-bind.sh BedrockTool.framework`

2) Add `BedrockTool.framework` to your Xcode app (Embed & Sign).

3) Example Swift usage (see `SampleViewController.swift`):
   - Set `PACKS_API_URL` / PACKS_API_TOKEN in your app before calling.
   - Call the binding to request packs; the binding returns either a URL (string)
     or a local path to a downloaded zip file. If the returned string is a file
     path, handle it with FileManager and unzip / present share sheet.

4) Notes:
   - The binding uses `PACKS_API_URL` environment variable; set it in your app
     scheme or supply your own wrapper that sets the URL.
   - For production, use TLS, certificate pinning, and protect `PACKS_API_TOKEN`.
