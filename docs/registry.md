# MCP Registry entry (`server.json`)

`server.json` at the repository root is the entry for the [official MCP Registry](https://registry.modelcontextprotocol.io) under the name `io.github.candidloom-hub/sourcebolt`. It follows the schema named in its `$schema` field (2025-12-11). `go test ./tools/distcheck` validates it against a pinned copy of that schema.

Nothing has been published. Publishing is an owner step (see the end of this page).

## Why it lists only the Docker image

The registry checks that every listed package exists and belongs to this server. Today only the GHCR image does:

- `ghcr.io/candidloom-hub/sourcebolt:0.3.1` carries the label `io.modelcontextprotocol.server.name=io.github.candidloom-hub/sourcebolt` (set in `.goreleaser.yaml`).
- The npm package `sourcebolt` is published (0.2.0) and listed in `server.json`. The PyPI package is built by the release workflow but not published yet, so listing it would fail the check.

The image entry passes `--mount type=volume,src=sourcebolt-config,dst=/config` so the encrypted sign-in profile survives between runs.

## Adding packages back

Bump the top-level `version`, then add the blocks below to `packages`. Each block is checked by `go test ./tools/distcheck` (it is added to the current `server.json` and validated), so keep them valid when you edit them.

### npm (published; already in `server.json`)

Ownership check: `mcpName` in `npm/launcher/package.json` must equal the server name (it does).

<!-- registry-package: npm -->
```json
{
  "registryType": "npm",
  "registryBaseUrl": "https://registry.npmjs.org",
  "identifier": "sourcebolt",
  "version": "0.3.1",
  "runtimeHint": "npx",
  "packageArguments": [{ "type": "positional", "value": "serve" }],
  "transport": { "type": "stdio" }
}
```

#### How npm publishing authenticates (owner setup)

The `npm` job in `.github/workflows/release.yml` runs `npm stage publish` with npm CLI 11.21.0. npm tries trusted publishing (OIDC) first for each package. When a package has no trusted publisher that matches, npm uses the `NPM_TOKEN` secret instead. Each version is staged either way, so you still approve it with 2FA on npmjs.com before it goes public.

Until you finish the steps below, publishing uses `NPM_TOKEN` as before. That token expires around 2026-11-19.

1. Add a trusted publisher to each of the 7 packages:
   - `sourcebolt`
   - `@candidloom-hub/sourcebolt-linux-x64`
   - `@candidloom-hub/sourcebolt-linux-arm64`
   - `@candidloom-hub/sourcebolt-darwin-x64`
   - `@candidloom-hub/sourcebolt-darwin-arm64`
   - `@candidloom-hub/sourcebolt-win32-x64`
   - `@candidloom-hub/sourcebolt-win32-arm64`

   For each one, sign in at npmjs.com and open the package page (for example `https://www.npmjs.com/package/sourcebolt`). Click **Settings**. In the **Trusted Publisher** section, under **Select your publisher**, click **GitHub Actions**, then fill in:

   | Field | Value |
   | --- | --- |
   | Organization or user | `candidloom-hub` |
   | Repository | `sourcebolt` |
   | Workflow filename | `release.yml` (the file name only, not `.github/workflows/...`) |
   | Environment name | `npm` |
   | Allowed actions | only `npm stage publish` (recommended), not `npm publish` |

   Click **Set up connection** (or **Save**) and confirm with 2FA.

2. Publish the next release with `publish_npm=true`. In the "Stage npm packages" step log, check that npm reports trusted publishing or OIDC, then approve the 7 staged versions on npmjs.com (platform packages first, `sourcebolt` last). Each version should show a provenance badge.
3. After one release has staged through OIDC, delete the token and lock the packages:
   - Run `gh secret delete NPM_TOKEN -R candidloom-hub/sourcebolt` (it is a repository secret). Then revoke the token on npmjs.com under **Access Tokens**.
   - For each package, open **Settings > Publishing access** and select **Require two-factor authentication and disallow tokens**. Trusted publishing keeps working after this, because the setting only applies to tokens.

If a step fails with `E404` or `ENEEDAUTH` after you delete the token, one package's trusted-publisher fields do not match. The usual causes are a typo in the workflow file name or a missing `npm` environment name.

### PyPI (after the wheels are published)

Ownership check: the PyPI README must contain `mcp-name: io.github.candidloom-hub/sourcebolt` (the repository README has it in an HTML comment).

<!-- registry-package: pypi -->
```json
{
  "registryType": "pypi",
  "registryBaseUrl": "https://pypi.org",
  "identifier": "sourcebolt",
  "version": "0.3.1",
  "runtimeHint": "uvx",
  "packageArguments": [{ "type": "positional", "value": "serve" }],
  "transport": { "type": "stdio" }
}
```

### MCP Bundle (after a release that has `.mcpb` assets)

The URL must be a GitHub release download and contain "mcp" (the file name does). `fileSha256` is the line for that file in `sourcebolt-mcp-checksums.txt` on the release. One block per platform you want to list; the hash below is a placeholder of the right shape and must be replaced.

<!-- registry-package: mcpb -->
```json
{
  "registryType": "mcpb",
  "identifier": "https://github.com/candidloom-hub/sourcebolt/releases/download/v0.3.1/sourcebolt-mcp-darwin-arm64.mcpb",
  "fileSha256": "0000000000000000000000000000000000000000000000000000000000000000",
  "transport": { "type": "stdio" }
}
```

## Publishing (owner only)

Not done by any automation in this repository. When approved:

```bash
mcp-publisher login github     # as the candidloom-hub account
mcp-publisher publish          # from the repository root
```

The registry is in preview and says data may be reset before general availability. Downstream directories (the GitHub MCP registry and VS Code's `@mcp` gallery, later PulseMCP) read from it; see [research/marketplaces.md](../research/marketplaces.md).
