# Live capture

Upstream implementations disagree on 40 request shapes (see `research/rpc-catalog.md` §10). We pin each one from a real NotebookLM session, saved as a sanitized golden fixture. Synthetic fixtures (`"synthetic": true`) are replaced as captures arrive.

## Rules

- The owner signs in to NotebookLM. Claude and other agents never type Google credentials.
- A HAR holds session cookies. Never commit it and never paste it into chat. `*.har` is git-ignored.
- Only the sanitized fixtures are committed. The converter drops all headers, cookies and the CSRF token, replaces every UUID with a stable fake id and every email with `user@example.com`, strips query strings from Google URLs, and replaces free text (titles, notes, answers) unless `--keep-titles` is passed.
- Use a throwaway test notebook with public sources for captures.

## Steps

1. Open Chrome or Edge, sign in to https://notebook.google.com, open DevTools → Network, tick "Preserve log".
2. Perform the actions for the checklist items you are capturing. P1 needs items 1–7:
   - Load the notebook list (home page).
   - Open a notebook.
   - Create a notebook, rename it, add a URL source, add a text source.
3. Right-click the request list → "Save all as HAR (sanitized)" → save outside the repo or as `capture.har` in the repo root (git-ignored).
4. Convert:

   ```bash
   go run ./tools/capture --har capture.har --out testdata/fixtures
   ```

5. Review the new `testdata/fixtures/<rpc>/live_*.json` files, then delete the HAR.
6. Run `go test ./...`. Where a live fixture contradicts a payload builder in `internal/nlm`, change the builder and record why in the commit.
7. Import cookies for live smoke tests: export cookies for `google.com` (cookies.txt or JSON) and run `sourcebolt login --import <file>`. Then:

   ```bash
   go test -tags live ./internal/nlm -run TestLiveListNotebooks -v
   ```

## Results

| Date | Host | Method | Calls | Fixtures |
|---|---|---|---|---|
| 2026-10-13 (UTC) | notebook.google.com | in-page XHR/fetch hook via Claude in Chrome on the owner's signed-in Brave; capture handed over through the clipboard (page CSP blocks localhost; download was blocked) | 41 | 21 (one per RPC id, three rLM1Ne states) |

Size reductions before export: `wXbhsf` kept 3 notebook rows; the chat stream's cumulative snapshots (167–207 KB each, 1.73 MB for one short answer, 11 chunks) were skeletonized (strings over 48 chars → `<str N>`, arrays cut to 6) so the structure survives without the content.

### Findings (rpc-catalog §10 items settled)

| Item | Result |
|---|---|
| 6 request-options cohort | Long form `[2,null,[1],[1,null×9,[1,3]]]` ("HDR") on wXbhsf, rLM1Ne, VfAZjd, CCqFvf, izAoDd, hPTbtc, khqZz, cFji9, I3xc3c, JFMDGd, e3bVqc, EylDcb and the chat stream. gArtLc/sqTeoe use `[2,null,null,[1,null×9,[1]],[[1,4,8,10,14,2,3,6]]]`. Short `[2]` is outdated. |
| 7 rLM1Ne last arg | `0` on first open, `1` while polling source processing; new trailing block `[[null,null,[]]]`. |
| wXbhsf | `[null,1,null,HDR,null,[[null,null,[]],[[]],[null,[]]]]`; row layout `[title, sources, id, emoji, null, meta]` confirmed. |
| izAoDd (URL) | spec `[null,null,[url],null×7,1]`, then notebook id, then HDR. Source status 5 while fetching, then 1/2. |
| CCqFvf | `["",null,null,HDR]`; response `[11][0][0]` is the default chat thread id. |
| 17 chat params | 9-element form confirmed: `[sources,question,null,HDR,conversation_id,null,null,notebook_id,seq]`. |
| 18 source nesting | `[[[sid]]]` (list of `[[sid]]`). |
| 20 conversation id | The notebook's default thread (from CCqFvf `[11]` / `hPTbtc`), sent on the first ask. |
| stream frames | `wrb.fr` rows with a **null** rpc id; answer row `[text, null, [conv_id, response_id, n], null, rich_text...]`. |
| hPTbtc | rows `[conv_id, created, updated]`. |
| new ids | `vB2zic` `["notebooks/<id>", 7]`, `sODAg` `[]` (artifact quota), `ozz5Z` with `[[[null,"1",627],...]]` fires on load (not add-URL). |
