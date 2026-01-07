iOS Build & deployment notes

Overview
--------
This repository includes:
- `mobile/` - a small Go package intended to be bound to iOS/Android via gomobile
- `scripts/gomobile-bind.sh` - helper to run `gomobile bind` on macOS
- `cmd/packsserver` - simple HTTP server which runs `bedrocktool packs <server>` and returns a ZIP containing generated `.mcpack` files
- `.github/workflows/ios-build.yml` - macOS workflow that builds the framework and optionally archives an IPA (requires signing secrets)

Important constraints
---------------------
- Building an iOS framework (gomobile bind -target=ios) and creating an IPA require **macOS** and Xcode. You cannot produce a signed IPA within a Linux Codespace.
- Signing an IPA requires Apple Developer certificates and provisioning profiles. Do NOT commit these to the repo; use GitHub repository secrets instead.

Local macOS build steps
------------------------
1. On a **macOS** machine (or GitHub macOS runner):
   - Install gomobile: `go install golang.org/x/mobile/cmd/gomobile@latest`
   - Initialize: `gomobile init`
   - Generate framework: `./scripts/gomobile-bind.sh build/BedrockTool.framework`
   - Add `BedrockTool.framework` to an Xcode app and embed (Embed & Sign)

2. To run `cmd/packsserver` locally:
   - `PACKS_API_TOKEN=secret go run ./cmd/packsserver -bedrock ./cmd/bedrocktool/bedrocktool`
   - From your app, set `PACKS_API_URL` to the server (e.g. `https://your-host:8080`) and call `RequestPacks(server)`.

CI (GitHub Actions)
-------------------
- The included workflow `ios-build.yml` is a starting point. Add these secrets to your repo settings:
  - `APPLE_CERT` (base64 encoded p12),
  - `APPLE_CERT_PASSWORD`,
  - `PROVISIONING_PROFILE` (base64 encoded mobileprovision file).
- The workflow builds the framework and uploads it as an artifact. If signing secrets are provided, it will attempt to build and export an IPA.

Security notes
--------------
- Use `PACKS_API_TOKEN` to protect `packsserver` endpoints and **serve only over TLS**.
- Validate inputs (server IPs/hosts) and rate-limit usage to prevent abuse.

Next actions
------------
- Add a minimal Xcode example project in `mobile/ios/Example` (requires macOS to create). If you want, I can add an example Xcode project structure and placeholders that you can open on a Mac and finish the signing settings there.