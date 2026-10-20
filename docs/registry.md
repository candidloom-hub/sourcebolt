# MCP Registry entry (`server.json`)

`server.json` at the repository root is the entry for the [official MCP Registry](https://registry.modelcontextprotocol.io) under the name `io.github.candidloom-hub/sourcebolt`. It follows the schema named in its `$schema` field (2025-12-11). `go test ./tools/distcheck` validates it against a pinned copy of that schema.

Nothing has been published. Publishing is an owner step (see the end of this page).

## Why it lists only the Docker image

The registry checks that every listed package exists and belongs to this server. Today only the GHCR image does:

- `ghcr.io/candidloom-hub/sourcebolt:0.3.0` carries the label `io.modelcontextprotocol.server.name=io.github.candidloom-hub/sourcebolt` (set in `.goreleaser.yaml`).
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
  "version": "0.3.0",
  "runtimeHint": "npx",
  "packageArguments": [{ "type": "positional", "value": "serve" }],
  "transport": { "type": "stdio" }
}
```

### PyPI (after the wheels are published)

Ownership check: the PyPI README must contain `mcp-name: io.github.candidloom-hub/sourcebolt` (the repository README has it in an HTML comment).

<!-- registry-package: pypi -->
```json
{
  "registryType": "pypi",
  "registryBaseUrl": "https://pypi.org",
  "identifier": "sourcebolt",
  "version": "0.3.0",
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
  "identifier": "https://github.com/candidloom-hub/sourcebolt/releases/download/v0.3.0/sourcebolt-mcp-darwin-arm64.mcpb",
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
