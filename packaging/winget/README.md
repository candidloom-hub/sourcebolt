# winget manifests

`manifests/c/candidloom-hub/sourcebolt/0.2.0/` holds a ready-to-submit manifest set for package id `candidloom-hub.sourcebolt`, version 0.2.0. It has **not** been submitted; submitting needs the owner's approval.

- Schema: 1.12.0, the version the [winget-pkgs pull request template](https://github.com/microsoft/winget-pkgs/blob/master/.github/PULL_REQUEST_TEMPLATE.md) asks for (checked 2026-10-21). Newer schema folders exist in winget-pkgs, but recent merged manifests still use 1.12.0.
- Layout: one version, one installer and one defaultLocale file, under `manifests/<first letter>/<publisher>/<package>/<version>`, as [doc/Authoring.md](https://github.com/microsoft/winget-pkgs/blob/master/doc/Authoring.md) requires.
- Installer: `InstallerType: zip` with a nested `portable` installer (`sourcebolt.exe`, command alias `sourcebolt`), x64 and arm64. winget unpacks the release zip, puts the exe in its portable-packages folder and adds a `sourcebolt` alias to PATH. No admin rights are needed.
- SHA-256 values: copied from the signed v0.2.0 `checksums.txt` and checked against it.
- `winget validate --manifest packaging\winget\manifests\c\candidloom-hub\sourcebolt\0.2.0` reports `Manifest validation succeeded.` (winget v1.29).

## Test it locally (optional)

`winget install --manifest` needs local manifests turned on, which is an admin setting:

```powershell
winget settings --enable LocalManifestFiles   # in an elevated PowerShell
winget install --manifest packaging\winget\manifests\c\candidloom-hub\sourcebolt\0.2.0
sourcebolt --version                          # in a new terminal: sourcebolt 0.2.0
winget uninstall candidloom-hub.sourcebolt
```

## Submit (owner only)

Either way opens a pull request to microsoft/winget-pkgs, which a Microsoft bot validates and a moderator merges. The first PR from an account also asks you to sign the Microsoft CLA in a PR comment.

**With wingetcreate** (it forks winget-pkgs for you; it asks for a GitHub sign-in or takes a token with `--token`):

```powershell
winget install Microsoft.WingetCreate
wingetcreate submit --prtitle "New package: candidloom-hub.sourcebolt version 0.2.0" packaging\winget\manifests\c\candidloom-hub\sourcebolt\0.2.0
```

**By hand:** fork microsoft/winget-pkgs, copy the `0.2.0` folder to `manifests/c/candidloom-hub/sourcebolt/0.2.0/` in your fork, commit, and open a pull request titled `New package: candidloom-hub.sourcebolt version 0.2.0`, ticking the checklist in the template.

## Later versions

`.goreleaser.yaml` has a `winget` section that writes the same three files for every release into `dist/winget/`. It opens a draft pull request from the `candidloom-hub/winget-pkgs` fork automatically only when the release workflow has a `WINGET_PKGS_TOKEN` secret (a token that can push to that fork and open pull requests on microsoft/winget-pkgs). Without that secret the upload is skipped and the release still succeeds. After the first version is merged, `wingetcreate update candidloom-hub.sourcebolt --version <new> --urls <x64 zip> <arm64 zip> --submit` is the manual alternative.

The GitHub release is created as a draft. Submit or let the bot open the pull request only after the draft is published, because winget's validation downloads the zip from the release URL.
