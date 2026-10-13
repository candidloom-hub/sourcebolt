# missdeer/notebooklm-client

**LICENSE WARNING: GPL-3.0 (`LICENSE:1-2`; npm manifests say `GPL-3.0-only OR LicenseRef-Commercial`, `npm/notebooklm/package.json:13`). The README's last line says "MIT" which contradicts both; treat the repo as GPL. This note records BEHAVIOR and FACTS only. No code was copied and none may be ported. Reimplement from behavior.**

- URL: https://github.com/missdeer/notebooklm-client
- Language / runtime: Go 1.26.1 (`go.mod:3`), cobra CLI, rod (browser), utls (TLS fingerprint), `modernc.org/sqlite` (Firefox/Safari cookie import). Note: all deps are marked `// indirect` in `go.mod` (module probably never tidied).
- License: GPL-3.0 (see above).
- Stars / last commit: 0 stars (per upstreams.md); HEAD `72dba99` 2026-06-25; 18 commits.
- Versions reviewed: v0.1.0 and v0.1.1 (both 2026-04-22); npm `@notebooklm-client/notebooklm-{darwin-universal,linux-arm64,linux-x64,win32-arm64,win32-x64}` + shim `@missdeer/notebooklm@0.1.1`.
- Lineage: a Go port of `icebear0828/notebooklm-client` (TypeScript); `SYNC_BASELINE.md` pins the TS commit `7319b6c` (2026-05-27, "add 5 missing RPC IDs") and the Python `jacob-bd` mcp-cli v0.7.3 (`6d41c75`, 2026-06-09) as reference for the CLI surface. The sync-baseline file is a good practice (see Adopt).
- Not an MCP server (no MCP code; grep found none).

## Access method (behavior)

Hybrid RPC + browser, with the split decided by a pluggable **Transport** interface (`internal/transport/transport.go:10-16`: `Execute(ctx, Request) -> body`, `GetSession()`, `RefreshSession(ctx)`, `Close()`).

Three transports, selected by `--transport auto|http|curl|browser` (README "Transport 模式"):

| Transport | What it is | TLS fidelity | Needs browser |
|---|---|---|---|
| `http` | pure Go using uTLS with a Chrome 131 ClientHello (`utls.HelloChrome_131`, `internal/transport/utls.go:173`) plus ALPN switch between HTTP/2 and HTTP/1.1 (`utls.go:181-213`) | "99% Chrome" (author's claim) | no |
| `curl` | subprocess `curl-impersonate` | 100% | no (needs binary) |
| `browser` | rod launches real Chrome/Edge/Brave/Chromium and the RPC `fetch` runs inside the page | 100% | yes |
| `auto` (default) | prefers non-browser; falls back to browser only if none of the others work | | no* |

Only `export-session` strictly needs a browser (README "必须启动浏览器"). Everything else is plain HTTPS. This confirms the split we plan: RPC first, browser as fallback and as login.

Claim worth testing (internal/client/client.go:123-127): "Google guards the dashboard HTML endpoint with cookie-to-TLS fingerprint binding, so plain net/http gets 302'd to accounts.google.com/CookieMismatch even with valid cookies." So token refresh (the `GET /` page fetch) is done with the uTLS client, not Go's default TLS. If true this contradicts tmc/nlm, LocalKinAI and Dokkabei97, which fetch the page with `net/http`. [unverified; cannot test without logging in]. Mitigation for us: make the TLS client pluggable (stdlib default, `refraction-networking/utls` optional) and detect `CookieMismatch` redirects.

Endpoints (`internal/rpc/constants.go:68-74`): host `notebooklm.google.com` (old host; the Go port is from April-June 2026, before tmc's July host switch), `.../_/LabsTailwindUi/data/batchexecute`, chat `.../google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed`, upload `.../upload/_/`.

## RPC catalog (IDs are facts; payload shapes not transcribed)

IDs in `internal/rpc/constants.go:3-55` (union with other repos is in the final summary):

CreateNotebook CCqFvf, ListNotebooks wXbhsf, GetNotebook rLM1Ne, RenameNotebook s0tc2d, DeleteNotebook WWINqb, RemoveRecentlyViewed fejl7e; AddSource izAoDd, AddSourceFile o4cbdc, GetSourceContent hizoJc, GetSourceSummary tr032e, DeleteSource tGMBJ, RefreshSource FLmJqe, CheckSourceFreshness yR9Yof, UpdateSource b7Wfje; CreateWebSearch Ljjv0c, CreateDeepResearch QA9ei, PollResearch e3bVqc, ImportResearch LBwxtb; GenerateArtifact R7cb6c, GetArtifactsFiltered gArtLc, DeleteArtifact V5N4be, RenameArtifact rc3d8d, GetInteractiveHTML v9rmvd, ExportArtifact Krh3pd, ShareArtifact RGP97b, ReviseSlide KmcKPe, GetSuggestedReports ciyUvf, GetStudioConfig sqTeoe; CreateNote CYK0Xb, GetNotes cFji9, GenerateMindMap yyryJe, UpdateNote cYAfTb, DeleteNote AH0mwd; ListChatThreads hPTbtc, GetConversationTurns khqZz, **DeleteChatThread J7Gthc** (tmc calls this DeleteChatTurns); GetShareStatus JFMDGd, ShareNotebook QDyure; GetAccountInfo ZwVcOc, SetUserSettings hT54vc, GetNotebookSummary VfAZjd, GetRecommendedTopics otmP3b, GetUIConfig ozz5Z (tmc: LogEvent), **ReportPlayProgress Fxmvse** (tmc: UpsertArtifactUserState).

Artifact type codes: Audio 1, Report 2, Video 3, Quiz 4, MindMap 5, Infographic 7, SlideDeck 8, DataTable 9 (`constants.go:57-66`), matching Dokkabei97/LocalKinAI (teng-lin lineage). A shared constant `DefaultUserConfig = [2,null,null,[1,null*9,[1]],[[2,1,3]]]` and `PlatformWeb=[2]` is the request-context fragment (`constants.go:76-80`) — note it differs from tmc's `[2,null,[1],[1,null*9,[1,3]]]`, another example of divergent context blocks.

**RPC-ID override file** (`internal/rpc/config.go:9-70`, `paths.go`): `~/.notebooklm/rpc-ids.json` is a map `{builtinId: overrideId}` read at startup (and reloadable); each call resolves its id through it. Lets a user patch a rotated RPC id without a new release. Good idea, see Adopt.

## Streaming/chat

Behavior only: chat posts to the streaming endpoint (not batchexecute) and a dedicated parser (`internal/parser/chat.go`) pulls answer text and citation data from the chunked stream; `chat --configure` sets persona/response style (commit `163aa44`). I did not transcribe wire details (GPL); tmc/nlm covers them with MIT code.

## Auth lifecycle (behavior)

- Session file `~/.notebooklm/session.json` (override dir with `NOTEBOOKLM_HOME`, inline JSON via `NOTEBOOKLM_AUTH_JSON`); fields: `at`, `bl`, `fsid`, `cookies` (flat string), `cookieJar` (structured, optional, with expiry), `userAgent`, `language` (`internal/types/types.go:165-173`). Same file format as the TypeScript original.
- **Two lifetimes** (README "Session 管理"): short-lived tokens `at/bl/fsid` live ~1-2 h; cookies live weeks to months. `refresh-session` GETs the dashboard with the cookies and re-extracts `SNlM0e` (`at`), `cfb2h` (`bl`), `FdrFJe` (`fsid`) and `<html lang>` by regex, merges `Set-Cookie` headers into the structured jar preserving Expires/Max-Age/deletion semantics, and rewrites `session.json` (`internal/session/refresh.go:1-110`). This is the standard pattern; adopt the cookie-jar merge.
- Auto-refresh: on session-expiry signal the client calls `RefreshTokens` once; a **RefreshGuard** (single-flight with shared result and retry-by-next-caller on failure) prevents N concurrent callers from refreshing simultaneously (`internal/util/refreshguard.go`). If refresh fails it reloads the file from disk in case another process refreshed it.
- **Expiry detection signal**: HTTP 200 with frame `["wrb.fr","<RPC>",null,null,null,[16],"generic"]` (gRPC UNAUTHENTICATED in-band) detected by a narrow regex (`internal/transport/authdetect.go`). Same signal tmc found. Also: 302 to `accounts.google.com/CookieMismatch`.
- Login: `export-session` launches rod on a persistent profile dir `~/.notebooklm/chrome-profile` (`rpc/paths.go`), with anti-automation hardening (commit `72dba99`: disable rod's leakless wrapper, drop `enable-automation`, `--disable-blink-features=AutomationControlled`, clear stale crash/DevTools state, create `about:blank` first, inject navigator/plugins/chrome/canvas/WebGL patches via `Page.addScriptToEvaluateOnNewDocument` before navigating). Leakless is disabled because it spawns an extra `leakless.exe` parent on Windows. This is an arms race; fragile and ToS-adjacent, so treat as optional.
- **`import-cookies` (no browser started)**: sources = Firefox `cookies.sqlite` (all profiles on Windows `%APPDATA%\Mozilla\Firefox\Profiles`, macOS `~/Library/Application Support/Firefox/Profiles`, Linux `~/.mozilla/firefox` plus XDG/snap/flatpak variants), macOS Safari `Cookies.binarycookies`, or any Netscape `cookies.txt`. Reads only `google.com`, `youtube.com`, `googleusercontent.com` domains. When the same cookie exists in several profiles the freshest wins (Firefox `lastAccessed` microseconds / Safari `creation_date`); prints per-source counts. Firefox details: copy DB first to dodge locks; `moz_cookies(host,name,value,path,expiry,isSecure,isHttpOnly,lastAccessed)`; **schema version >= 16 (Firefox 142+) stores `expiry` in milliseconds, earlier in seconds; `lastAccessed` is always microseconds** (`internal/session/import_firefox.go:33-95`). Firefox cookies are not encrypted, which is why this works on every OS.
- **Chromium family: explicitly not supported.** README: "Chrome 127+ App-Bound Encryption requires Chrome itself to decrypt"; workaround is a browser extension ("Get cookies.txt LOCALLY") or `yt-dlp --cookies-from-browser chrome --cookies cookies.txt` and then `import-cookies --file`. This is the clearest statement in these repos about ABE and it matches our Windows reality.
- Multi-account: not handled in code I read. [unverified]

## Reliability (behavior)

- Retry/backoff exists for artifact **downloads** (up to maxRetries with `attempt*10 s` sleeps while the CDN is not ready) and metadata polling with a timeout (`internal/download/download.go:39-158`). I found no retry in the RPC caller or transport. `util.JitteredDelay` (+-30% jitter) is used for human-like spacing (`internal/util/humanize.go`).
- Errors typed in `internal/types/errors.go` (`SessionError` etc.).
- Proxy support everywhere (SOCKS5/HTTP/HTTPS with `user:pass@`, env `SOCKS5_PROXY`... ; the rod transport cannot pass proxy credentials through `--proxy-server`).

## Performance notes

Single static binary; no runtime; `auto` transport avoids launching a browser. npm packaging: platform sub-packages + a Node shim `run.js` that resolves `process.platform`/`arch` to `@notebooklm-client/notebooklm-<plat>` and execs the binary (`npm/notebooklm/run.js:12-28`), same pattern CLAUDE.md wants for our npm distribution.

## MCP tools

None.

## Tests and quality

`session_test.go`, `session_import_test.go`, `import_discovery_test.go`, `import_firefox_test.go`, `import_netscape_test.go`, `import_safari_test.go`, `store_test.go`, `cookies_test.go`, `types_test.go`, `rod_test.go` (launcher flags). CI: ubuntu only, `go mod verify`, vet, build, test (`.github/workflows/ci.yml`). No RPC/parser golden tests visible in the file list (`internal/parser` has no `_test.go`) [file list check].

## Versions

v0.1.0 and v0.1.1 on 2026-04-22; 18 commits: 04-22 HTTP/2 transport, npm shims; 04-23 proxy flags, cookie-based session bootstrap and token expiry visibility; 06-14 sync to TS baseline, many new CLI commands; 06-25 rod hardening.

## Adopt (ideas only, reimplement)

1. Transport interface with `Execute`, `GetSession`, `RefreshSession`, `Close` and an `auto` selection policy (RPC first, browser only on demand).
2. Single-flight refresh guard + "reload session from disk if refresh fails" (another process may have refreshed).
3. Cookie jar with expiry that merges `Set-Cookie` after every refresh; store both flat string and structured jar.
4. Firefox `cookies.sqlite` import (cross-platform, unencrypted) including schema-16 millisecond expiry rule; Netscape `cookies.txt` import; "freshest cookie wins across profiles"; per-source counts printed. Gives a Windows path that avoids ABE.
5. `rpc-ids.json` override file so users can hot-patch rotated RPC IDs.
6. A `SYNC_BASELINE.md` recording which upstream commits we have synced and what remains (`SYNC_BASELINE.md:1-60`).
7. npm per-platform binary packages with a tiny launcher.
8. Document the ABE limitation honestly and point to CDP-attach or exported `cookies.txt`.

## Avoid

1. GPL code reuse (all of it).
2. Stealth/anti-detection JS injection as a default path.
3. Ignoring TLS fingerprint issues silently; make it a diagnosed condition (CookieMismatch).
4. `go.mod` with every dependency marked indirect; run `go mod tidy`.
5. No retry on RPC calls.

## Evidence

Read: `README.md`, `SYNC_BASELINE.md`, `go.mod`, `internal/rpc/{constants,config,paths}.go`, `internal/session/{refresh,import_firefox}.go` (partial), `internal/client/client.go:100-145`, `internal/transport/{transport,authdetect,headers}.go`, `internal/util/{refreshguard,humanize}.go`, `internal/download/download.go` (grep), `internal/types/types.go:165-190`, `git show 72dba99`, npm manifests. No code reproduced.
