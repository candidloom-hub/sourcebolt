# Documentation best practices

Date: 2026-10-20. Sources were read with a web fetch on that date. This file lists the practices the user docs adopt and where each one comes from.

## Sources studied

| Project | Link | What stood out |
|---|---|---|
| REA | https://github.com/morluto/rea | One-line value statement, a row of jump links, one install command up front, a scope table with requirements per target, FAQ that answers objections, explicit limits, provenance warning. |
| Awesome (guidelines) | https://github.com/sindresorhus/awesome/blob/main/awesome.md and https://github.com/sindresorhus/awesome/blob/main/pull_request_template.md | Succinct objective description at the top, a contents list, consistent entry style, explicit license, a contributing guide, extra notices in a footer. |
| ripgrep | https://github.com/BurntSushi/ripgrep | "Why should I use" and "Why shouldn't I use" sections, benchmarks with the exact command and the machine, candid notes where the tool is not faster, install grouped by platform, deep links into a guide and FAQ. |
| uv | https://github.com/astral-sh/uv | Tagline, short highlights list, separate install blocks for macOS/Linux and Windows PowerShell, link to full benchmark data, acknowledgements of the projects it learned from. |
| fzf | https://github.com/junegunn/fzf | Demo image near the top, table of contents, install per channel, upgrade notes, Sponsors section linking to GitHub Sponsors. |
| gum (Charm) | https://github.com/charmbracelet/gum | Demo GIF near the top, tutorial by example, release binaries and `go install` listed with the package managers. |
| Astro docs style guide | https://contribute.docs.astro.build/guides/writing-style/ | Short sentences, active voice, imperative steps, simple words, spell out acronyms on first use, do not talk down to the reader, no "we/let's" in instructions, H2 for sections, code formatting for commands. |

## Practices adopted

1. **One-line value statement, then a 30-second "what it does".** (REA, uv, gum) The README opens with one sentence and a short paragraph, then a demo image.
2. **Jump links under the title.** (REA, ripgrep) Readers go straight to Quick start, Install, Clients, Docs.
3. **Quick start in five steps or fewer.** (REA, uv) Install, sign in, connect a client, ask a question. Each step is a copy-paste command.
4. **Separate PowerShell and bash blocks only where they differ.** (uv) Commands that are the same on every OS appear once.
5. **Honest availability per install channel.** (ripgrep "why not", REA limits) A table says which channels work today and which are "not published yet".
6. **Measured numbers only, with the machine and the command.** (ripgrep) Every number links to `bench/RESULTS.md`; unmeasured items say "not measured".
7. **"Why use it" and "why you might not".** (ripgrep) `docs/why-sourcebolt.md` lists strengths and also cases where another project fits better; `docs/comparison.md` shows where others win.
8. **Progressive disclosure.** (REA, ripgrep, uv) The README stays short. Tool lists are grouped by task and link to the generated `docs/tools.md`. Details live in `docs/`.
9. **FAQ that answers objections.** (REA, uv) Is it safe? Does it upload my files? Is it official? Is it free?
10. **Troubleshooting keyed by the exact error text.** (REA's "update first, then report") Each entry quotes the real error code or message from the code and gives the fix.
11. **Plain English.** (Astro) Short sentences, active voice, imperative steps, acronyms spelled out on first use (MCP, RPC, DOM, SSRF), no "we/let's" inside steps.
12. **Credits and acknowledgements.** (uv, Awesome footnotes) Upstream projects are credited with their licenses; NOTICE obligations are kept.
13. **Support section deferred.** (fzf has a Sponsors section.) Donations are not open yet (owner decision, 2026-10-20), so the README has no Sponsor section; `docs/sponsor.md` is linked only from the docs home as "not open yet".
14. **Contributing and License near the end.** (Awesome, gum) Short sections that link to the files.
15. **A docs home page.** (REA documentation index) `docs/index.md` lists every page with one line each.
16. **Cite competitor facts with links and a check date.** (Awesome: rationale per entry) `docs/comparison.md` links every fact to the repo and says when it was checked; uncertain facts are marked.

## Not adopted

- Translated READMEs and community chat badges (REA, fzf): no translations or chat channel exist.
- CI status badges at the top (the Awesome template discourages them): the release and license badges carry more meaning for users.
- A hosted docs site (uv, Astro): the docs stay as Markdown in the repo for now.
