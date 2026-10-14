# Security

## What is at stake

sourcebolt holds your Google session cookies. Anyone who gets them can act as you in Google for as long as the session lives. The design goal is that the cookies exist only encrypted on disk, only in memory while serving, and never in logs, tool output or chat.

## Threat model

| Threat | Mitigation | Residual risk |
|---|---|---|
| Cookies read from disk | Profiles are AES-256-GCM encrypted; the key is in the OS keyring. Without a keyring the key is in a `0600` file next to the profile, which protects against other users, not against malware running as you. | Malware running as your user can use the keyring or key file. Container volumes should be protected like secrets. |
| Cookies or tokens in logs and errors | `safety.Redact` masks request bodies, CSRF tokens, session cookies and `SAPISIDHASH` values in every log line or error that may carry request data. | A new log site that skips `Redact` would leak; keep new logging behind it. |
| The model gets or leaks credentials | No tool accepts, returns or types credentials. `auth_login` and `auth_logout` over MCP return an error that names the CLI command. Login happens only on the CLI, in a browser you drive. | You must not paste cookie files into a chat. |
| SSRF through `source_add` URLs | URLs are checked before sending: public `http(s)` only; loopback, private, link-local, CGNAT, reserved, documentation, 6to4 and NAT64-embedded private addresses are rejected, as are numeric-host tricks (`2130706433`, `0x7f000001`, `127.1`). The URL that was validated is the URL that is sent. | Google fetches the URL, not your machine, so the main risk is policy, not your network. |
| Path traversal and special paths | Local paths must be absolute, inside allowed roots, free of `..`, and are symlink-resolved. On Windows, device paths, UNC paths, reserved names (`CON`, `NUL`, `COM1`...), alternate data streams and junctions are rejected. The checker (`internal/safety`) exists for file-based tools; no current tool reads local files (file upload is not enabled yet). | |
| Network exposure of the server | stdio is the default. `serve --http` accepts loopback addresses only, requires `--token-file` with a token of at least 16 characters, compares it in constant time, and rejects requests whose `Origin` is not `http://127.0.0.1` or `http://localhost`. | Any local process that has the token can use your notebooks. Keep the token file private. HTTP is plain, not TLS; it is loopback only. |
| Accidental destructive action by an agent | `delete` previews unless `confirm:true`, resolves names strictly and reports per-id outcomes; `share_update` previews when it widens access or removes people. | The agent can still confirm; review previews. |
| Debugging port during `login --attach` | The browser uses a dedicated profile, not your daily one; you close it afterwards. | While open, local processes can control that browser. |
| Hostile content in notebooks | Tool results are data from your sources and from Google; clients should treat them as untrusted. sourcebolt does not execute any of it. | Prompt injection through source text is a client-level risk. |
| Supply chain | Release artifacts are built by GoReleaser with checksums, a keyless cosign signature over the checksum file, CycloneDX SBOMs and signed images (configured in `.goreleaser.yaml`; not yet exercised by a release). | Verify checksums and signatures once a release exists. |

## What sourcebolt does not do

- It does not read cookies from your normal browser profiles or decrypt browser cookie stores.
- It does not use Google master tokens or the Android API.
- It does not send your data anywhere except to the NotebookLM host (and to URLs you asked it to add as sources, via Google).
- It does not bypass Google's quotas, CAPTCHAs or bot checks.

## Operational advice

- Run `sourcebolt status` and `sourcebolt doctor` to see where credentials live.
- Never commit config directories, cookie exports or HAR files. A HAR holds session cookies.
- If a cookie file or HAR was exposed, run `sourcebolt logout`, then sign out of other Google sessions and change your password.
- Prefer a Google account you can afford to lose access to while the project is unofficial.

## Reporting a vulnerability

Please report privately and do not open a public issue with exploit details. Use GitHub's private vulnerability reporting on https://github.com/candidloom-hub/sourcebolt (Security tab, "Report a vulnerability"). Include the version (`sourcebolt --version`), OS, and steps to reproduce, and remove any cookies or tokens from what you send. This channel only works once the repository's private vulnerability reporting is enabled; the owner should confirm that before the repository goes public.
