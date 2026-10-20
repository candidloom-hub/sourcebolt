# Security

## What is at stake

sourcebolt holds your Google session cookies. Anyone who gets them can act as you in Google for as long as the session lives. The design goal: the cookies exist only encrypted on disk and in memory while sourcebolt runs, and never in logs, tool output or chat.

## Threat model

| Threat | How sourcebolt handles it | What is left |
|---|---|---|
| Cookies read from disk | Profiles are encrypted with AES-256-GCM. The key is in the OS keyring. Without a keyring, the key is in a file readable only by you (`0600`) next to the profile. This protects against other users, not against malware running as you. | Malware running as your user can use the keyring or the key file. Protect container volumes like secrets. |
| Cookies or tokens in logs and errors | Every log line and error that may carry request data passes through a redaction step that masks request bodies, CSRF tokens, session cookies and `SAPISIDHASH` values. | A new log line that skips redaction would leak. New logging must go through it. |
| The model gets or leaks credentials | No tool accepts, returns or types credentials. `auth_login` and `auth_logout` over MCP return an error that names the CLI command. Sign-in happens only on the command line, in a browser you control. | Do not paste cookie files into a chat. |
| SSRF (server-side request forgery) through `source_add` URLs | URLs are checked before sending: public `http(s)` only. Loopback, private, link-local, CGNAT, reserved, documentation, 6to4 and NAT64-embedded private addresses are rejected, as are numeric-host tricks (`2130706433`, `0x7f000001`, `127.1`). The URL that was checked is the URL that is sent. | Google fetches the URL, not your machine, so the main risk is policy, not your network. |
| Path traversal and special paths | Local paths must be absolute, inside allowed folders, free of `..`, and are resolved through symlinks. On Windows, device paths, UNC paths, reserved names (`CON`, `NUL`, `COM1`...), alternate data streams and junctions are rejected. The checker exists for file-based tools; no current tool reads local files (file upload is not enabled yet). | |
| Network exposure of the server | stdio is the default. `serve --http` accepts loopback addresses only, requires `--token-file` with a token of at least 16 characters, compares it in constant time, and rejects requests whose `Origin` is not a loopback address. | Any local process that has the token can use your notebooks. Keep the token file private. HTTP mode is plain HTTP, not TLS; it is loopback only. |
| An agent deletes or shares something by accident | `delete` previews unless `confirm: true`, matches names strictly and reports the result per item. `share_update` previews when it widens access or removes people. | The agent can still confirm. Read previews before you approve them. |
| Debugging port during `login --attach` | The browser uses a separate profile, not your everyday one, and you close it afterwards. | While it is open, local programs can control that browser. |
| Hostile content in notebooks | Tool results are data from your sources and from Google; AI apps should treat them as untrusted. sourcebolt does not execute any of it. | Prompt injection through source text is a risk at the AI app level. |
| Supply chain | Release archives are built by GoReleaser in GitHub Actions, with SHA-256 checksums, a keyless cosign signature over the checksum file and a CycloneDX SBOM per archive. | Verify downloads (below). |

## Verify a download

Each release has `checksums.txt`, its signature bundle `checksums.txt.sigstore.json`, and one `.cdx.json` SBOM (software bill of materials) per archive.

**1. Check the signature of `checksums.txt`** with [cosign](https://docs.sigstore.dev/cosign/system_config/installation/). This proves the file was produced by this repository's release workflow:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/candidloom-hub/sourcebolt/.github/workflows/release.yml@refs/tags/v0.1.0" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

PowerShell (same command on one line):

```powershell
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json --certificate-identity "https://github.com/candidloom-hub/sourcebolt/.github/workflows/release.yml@refs/tags/v0.1.0" --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

It prints `Verified OK` on success. This exact command was run against the v0.1.0 `checksums.txt` and returned `Verified OK`.

For a later release, change the tag in `--certificate-identity`, or accept any tag from this repository with the regular-expression form:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/candidloom-hub/sourcebolt/' \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

In PowerShell, put the command on one line.

**2. Check your archive against `checksums.txt`.**

bash (Linux; on macOS use `shasum -a 256 -c --ignore-missing checksums.txt`):

```bash
sha256sum --ignore-missing -c checksums.txt
```

PowerShell:

```powershell
$f = "sourcebolt_0.1.0_windows_amd64.zip"
$want = (Select-String -Path checksums.txt -Pattern $f).Line.Split(" ")[0]
$got = (Get-FileHash $f -Algorithm SHA256).Hash.ToLower()
if ($got -eq $want) { "OK" } else { "MISMATCH" }
```

## What sourcebolt does not do

- It does not read cookies from your everyday browser profiles or decrypt browser cookie stores.
- It does not use Google master tokens or the Android API.
- It does not send your data anywhere except to the NotebookLM host (and, through Google, to URLs you ask it to add as sources). It has no telemetry.
- It does not bypass Google's quotas, CAPTCHAs or bot checks.

## Practical advice

- Run `sourcebolt status` and `sourcebolt doctor` to see where your session is stored.
- Never commit config folders, cookie exports or HAR files (browser network recordings). A HAR file contains session cookies.
- If a cookie file or HAR file was exposed: run `sourcebolt logout`, sign out of other Google sessions on your Google account's security page, and change your password.
- While the project is unofficial, prefer a Google account you can afford to lose access to.

## Reporting a vulnerability

Please report privately; do not open a public issue with exploit details. Use GitHub's private vulnerability reporting: open https://github.com/candidloom-hub/sourcebolt/security and choose **Report a vulnerability**. Include the version (`sourcebolt --version`), your operating system and the steps to reproduce. Remove any cookies or tokens from what you send.
