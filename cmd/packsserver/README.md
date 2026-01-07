packsserver

A small HTTP helper that runs `bedrocktool packs <server>` on the host and returns a zip containing generated `.mcpack` files.

Usage:
  PACKS_API_TOKEN=secret go run ./cmd/packsserver -bedrock ./cmd/bedrocktool/bedrocktool

Endpoints:
- POST /packs  JSON body {"server":"<ip-or-host>"}
  - If PACKS_API_TOKEN is set, include header X-API-Token: <token>
  - Responds with `application/zip` containing .mcpack files

Security: run this service on a trusted host, behind TLS, and protect with a token/firewall.