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

| Date | Host | Items captured | List notebooks RTT p50 / p95 |
|---|---|---|---|
| (pending) | | | |
