# Kisara-k/notebooklm-kit

- URL: https://github.com/Kisara-k/notebooklm-kit (fork of photon-hq/notebooklm-kit)
- Language / runtime: TypeScript SDK (same as upstream) plus a Python "pipeline" (Jupyter notebooks, patchright login, per-source batch generation/download)
- License: MIT inherited (package.json); fork adds no license file of its own [LICENSE present, not diffed].
- Stars / last commit: not queried; HEAD f3ac898 2026-09-12 "Merge pull request #1 from Kisara-k/patch/new-notebooklm-url"
- Versions reviewed: HEAD, package.json still 2.2.0; no tags (git tag empty). 172 commits; 30 by Kisara Kodithuwakku (2026-05-30..2026-09-12), the rest upstream photon history. `git diff aa44b49 HEAD --stat`: 26 files, +6210/-94, of which src/ is 7 files, +146/-86.
- REA ran (311 KB, same unreadable node output as upstream). All facts from `git diff aa44b49 HEAD` (aa44b49 = last upstream commit) and direct reading.
- Read together with photon-hq__notebooklm-kit.md; only the deltas are recorded here.

## What the 30 commits add (src/ deltas)

1. Host move (commit bc32922, 2026-09-12 "update notebooklm url to notebook", PR merged f3ac898): every `notebooklm.google.com` becomes `notebook.google.com`: batchexecute `host` (src/rpc/rpc-client.ts), `Origin`/`Referer` headers (rpc-client.ts, auth/refresh.ts SAPISIDHASH origin, `fetch('https://notebook.google.com/')` for gsessionid), the streaming URL `https://notebook.google.com/_/LabsTailwindUi/data/google.internal.labs.tailwind.orchestration.v1.LabsTailwindOrchestrationService/GenerateFreeFormStreamed` (src/utils/streaming-client.ts:612), auth page navigation + cookie URL (`context.cookies('https://notebook.google.com')`, pipeline/login.py), share URLs `https://notebook.google.com/notebook/{id}`. The service path `/_/LabsTailwindUi/data/batchexecute`, RPC ids, `bl`, `f.sid` constants and all payloads are UNCHANGED in that commit. This is direct code evidence that the post-rebrand production host is `notebook.google.com` with the same internal service path, and that the SAPISIDHASH origin and `Origin` header must match the new host. Consistent with roomi-fields 3.0.4 (host/cookie mismatch). The READMEs of moodRobotics claim "same API surface"; that holds for paths and ids, but the host differs.
2. Live-verified Studio payload fixes (commit 8ec0d05 and 7901749; src/services/artifacts.ts, src/types/artifact.ts). The author captured live `R7cb6c` requests ("Verified via live R7cb6c capture", probe prompt string "XXPROBE_PROMPT_77QQ"):
   - Slide deck options live at content index 16, shape `[[instructions, language, format, length]]`; the earlier SDK wrote index 13 where "the server silently ignored it". Format 2 = Presenter, 3 = Detailed deck. Length 2 = Short (5-10 slides), 3 = Default (10-15) (was 1/2). Default now 3.
   - Infographic options at index 14, shape `[[instructions, language, null, orientation, levelOfDetail, 1]]` (slots 0/1 had been swapped, language in slot 0; trailing constant `1` "purpose unknown"). Orientation 1 landscape, 2 portrait, 3 square; detail 1 concise, 2 standard, 3 detailed.
   - Video: `instructions` takes priority over `focus`; format enum widened to 1 Explainer, 2 Brief, 3 Cinematic; visual style enum re-based to 1-based (1 auto, 2 custom, 4 heritage, 9 kawaii; others marked `?` unresolved).
   - These match roomi-fields' `studio-rpc.ts` (presentation index 16 `[[focus, lang, 1, 3]]`, infographic index 14 `[[focus, lang, null, 1, 2, 1]]`) and contradict the index-6 layouts in moodRobotics. Prefer this 16/14 layout.
   - Quiz vs flashcards both return API type 4; they are distinguished by a heuristic that falls back to QUIZ; explicit caller type now overrides it (artifacts.ts parse). Our Go decoder should read the variant code directly.
3. Artifact timestamps: scan the raw row for `[seconds, nanos]` pairs (seconds > 1e9) from index 5 on; min = createdAt, max = updatedAt; positions vary by type ("[10]/[15] for QUIZ/FLASHCARDS, [8]/[13] for VIDEO") (artifacts.ts parse diff).
4. Double-encoded responses: artifact data may arrive as a JSON string; parse it, and match image URLs that contain `=w` / `=s` escapes as well as `=w`/`=s` (slide image URL scraping: `lh3.googleusercontent.com/notebooklm...=w/s<size>`).
5. Slide/infographic download: pass cookies as a raw `Cookie` extra HTTP header instead of `context.addCookies()` because Chrome CDP rejects cookie objects with "Invalid cookie fields" for certain Google cookie value formats (artifacts.ts diff, context creation with `extraHTTPHeaders`).
6. Python `pipeline/` (login.py, _core.py, _artifacts.py, _sources.py, config.py): patchright persistent profile per named profile (`pipeline/profiles/<name>`), `credentials[.<profile>].json` stores cookies only; the CSRF token `SNlM0e` is fetched fresh at each run by `GET https://notebook.google.com/` with the saved cookie header and regex `"SNlM0e"\s*:\s*"([^"]+)"` (pipeline/_core.py:38-60). Account switching = named profiles + `--logout` which deletes profile dir and credentials (commit a6221b1, 0b2660b).
7. Batch workflows (per-source flashcards/videos/artifacts, dry run, jobs files `jobs/<type>/<ts>_jobs.json` that can be resumed, outputs dir) in notebooks; CHANGELOG.md describes them (flashcard `numberOfCards` 1 fewer, 2 standard; difficulty 1/2/3).

## Access method

Same as photon-hq: internal batchexecute RPC + streaming endpoint over fetch, plus patchright/Playwright for login and for media download of slides/infographics (needs browser context for authenticated image URLs).

## RPC catalog

No new ids vs photon-hq/notebooklm-kit (see that note). Endpoint now `https://notebook.google.com/_/LabsTailwindUi/data/batchexecute` (rpc-client.ts:36 host). `bl` and `f.sid` constants are unchanged from upstream (2025-01 / 2026-01 build labels), and the fork's author reports working generation, download, list in 2026-09, which suggests the server does not reject stale `bl` [authors' behavior; not verified by me].

## Streaming/chat

Unchanged from upstream except host (streaming-client.ts:612). See photon note for body shape.

## Auth lifecycle

TS side: unchanged (env tokens, credentials.json, signaler refresh) except host constants. Python side: persistent patchright profile, cookies-only credential file, fresh CSRF per run (above). `.env.example` now tells users to read `window.WIZ_global_data.SNlM0e` on https://notebook.google.com. Multi-account via `--profile`.

## MCP tools

None (no MCP server).

## Versions

No tags; package version 2.2.0 unchanged. Timeline: 2026-05-30 first fork commits (automation notebooks, list/rename artifacts and sources, artifact download); 2026-06-01/02 slide deck download, file naming, utf-8, logout; 2026-06-12 profiles; 2026-09-12 host switch and notebooks "before applying changes for new notebooklm API" (6b35924).

## Adopt

1. Treat `notebook.google.com` as primary host with `/_/LabsTailwindUi/...` unchanged; pass the host into `Origin`, `Referer`, SAPISIDHASH origin and the cookie URL consistently (bc32922). Probe both hosts at startup like roomi-fields.
2. Studio payload layouts verified by live capture (slides idx 16, infographic idx 14, enums above) - strongest wire evidence in this batch; record as golden fixtures.
3. Probe-string capture technique ("XXPROBE_PROMPT_77QQ") to locate a field's slot in a real request: use in our capture tooling.
4. Fresh `SNlM0e` per run from cookie-only credentials (pipeline/_core.py:38-60): store cookies only, derive tokens.
5. Timestamp pair scanning and double-decode handling (artifacts.ts).
6. Resumable job files for long batch generation (jobs/*.json) as a pattern for our long-running Studio tasks.
7. Send cookies as header to a headless fetch context rather than `addCookies` to dodge CDP "Invalid cookie fields" (relevant to our browser fallback).

## Avoid

1. Python-notebook-centric design, not a library or server; business logic lives in `.ipynb` and Python shelling to `tsx` (pipeline/_artifacts.py builds TS source strings).
2. Leaving `?` placeholders in documented enums (artifact.ts visual style) and unknown trailing flags.
3. Committing notebook outputs (strip_nb_metadata.py exists to clean them; commit 6b35924 "commit notebooks before applying changes").
4. Everything inherited from photon-hq (plaintext credentials, hard-coded f.sid/bl, broken file upload).
5. Host changed by a global search-and-replace with no probing/fallback to the old host.

## Evidence

`git log` (authors, dates), `git show bc32922` (full diff), `git show 8ec0d05 --stat`, `git diff aa44b49 HEAD -- src/services/artifacts.ts src/types/artifact.ts` (filtered), `git grep` for host strings in src, pipeline/login.py:1-60, pipeline/_core.py:25-90, CHANGELOG.md:1-30.
