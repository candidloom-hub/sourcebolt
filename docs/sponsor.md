# Sponsor sourcebolt

sourcebolt is free and open source under the Apache-2.0 license. There is no paid tier and there will not be one. If it saves you time, you can support the work through [GitHub Sponsors](https://github.com/sponsors/candidloom-hub).

## What sponsorship pays for

- **Keeping up with NotebookLM.** NotebookLM has no public API. sourcebolt uses the same internal web calls the NotebookLM site makes, and Google changes them without notice. Each change means a new live capture, new fixtures and a fix release. This is most of the ongoing work.
- **Maintenance.** Dependency and security updates, Go and MCP SDK upgrades, bug reports, and keeping the release pipeline (binaries, Docker images, packages) working on every platform.
- **Performance work.** The benchmark suite in `bench/` runs on every change. Known open item: uncached tool overhead is above its 2 ms target.
- **Features that are still "coming soon".** More Studio generation kinds, Drive and file sources, and the other options listed under Known limitations in the [changelog](../CHANGELOG.md). Each one needs a live capture before it can ship.

Sponsorship does not buy priority over security fixes, and it does not change the license.

## Suggested tiers

A short list the owner can adopt on the GitHub Sponsors page. Amounts are suggestions only.

| Tier | Monthly | What you get |
|---|---|---|
| Thanks | $3 | Our thanks, and your name on the Sponsors page if you want it |
| User | $10 | The above, plus a mention in release notes for the month |
| Heavy user | $25 | The above, plus your feature requests get a written answer on whether and when they fit |
| Team | $100 | The above, plus your logo (small) in the README sponsor section, if one is added |

One-time sponsorships of any amount are welcome too.

## Other ways to help

- Report a breakage as soon as you see one, with `sourcebolt --version` and the error code (never paste cookies or tokens).
- Star the repository or tell someone who uses NotebookLM.
- Send a pull request. Small fixes and documentation help a lot.
