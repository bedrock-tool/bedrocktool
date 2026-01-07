mobile package

This package is intended to be bound with gomobile and used from iOS/Android.
It provides a small API to request resource packs for a Minecraft Bedrock server by
delegating the work to a remote HTTP service (see `cmd/packsserver`).

Configuration (env vars used by the mobile binding when executing on the device):
- PACKS_API_URL: https://your-server.example.com  (required)
- PACKS_API_TOKEN: optional API token to include with requests

Function:
- RequestPacks(server string) (string, error)
  - Sends a POST /packs {"server":"<server>"} to PACKS_API_URL
  - On success the response is JSON {"url":"https://.../download.zip"}
  - The return string is the URL you can download from (app should fetch and store it locally)

Security note: keep PACKS_API_TOKEN secret and implement network security in your iOS app (TLS pinning if needed).