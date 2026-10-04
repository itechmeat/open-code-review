# Fork release guide

This repository is a fork of [alibaba/open-code-review](https://github.com/alibaba/open-code-review). It is published to npm as `@itechmeat/open-code-review`, with the platform binaries in `@itechmeat/ocr-<os>-<arch>`, and as GitHub Release assets on this repository.

## Upstream-first policy

1. Upstream is the source of truth. The fork tracks it and carries a small set of changes on top.
2. The diff against upstream stays small and mechanical: values are edited in place, files are not reorganised, and fork-only prose lives in this file and in the "About this fork" note of each README.
3. The fork is rebased onto upstream release tags, not merged. Each fork change stays a clean commit that can be re-applied or offered upstream on its own.
4. Fixes that are useful to every user go upstream as their own pull request. The fork carries them only until they arrive through a rebase.
5. The website at open-codereview.ai, the IDE extensions, the composite GitHub Action and the CI examples are upstream's. The fork leaves them unchanged, so their install commands still name the upstream package.

## Versions and tags

1. A fork release is tagged `v<upstream version>-fork.<n>`, for example `v1.12.11-fork.1`. `<upstream version>` is the upstream tag the fork is rebased onto; `<n>` starts at 1 and grows with each fork release on the same base.
2. The npm version is the tag without the leading `v`. It is a valid semver prerelease, and it sorts as expected:
   1. `1.12.11-fork.10` is newer than `1.12.11-fork.2`, because numeric identifiers compare numerically.
   2. Any `1.12.12-fork.<n>` is newer than every `1.12.11-fork.<n>`.
3. The release workflow (`.github/workflows/release.yml`) runs only for tags of the exact form `vX.Y.Z-fork.N` (filter `v[0-9]+.[0-9]+.[0-9]+-fork.[0-9]+`). Pushing an upstream tag such as `v1.12.11`, or a suffixed tag such as `v1.12.11-fork.1-rc.1`, never publishes anything.
4. Every fork version is a prerelease, so the workflow publishes with `npm publish --tag latest`. npm refuses to publish a prerelease without an explicit tag. An explicit `--tag latest` also turns off npm's check against moving `latest` to a lower version, so release only in increasing version order: never tag an older base after a newer one.
5. Consequences for users:
   1. `npm install -g @itechmeat/open-code-review` installs the version that carries the `latest` dist-tag, which is always the newest fork release.
   2. A semver range such as `@itechmeat/open-code-review@^1.12` does not match prereleases and therefore matches no fork release. Pin an exact version (`@1.12.11-fork.1`) or use `@latest`.
   3. The launcher's background update check installs newer fork releases itself with `npm i -g`, including a newer `-fork.<n>` on the same upstream base. It writes an update hint (`~/.opencodereview/update-available`) only when that install fails.
6. GitHub Releases are not marked as prereleases. `install.sh` and `install.ps1` resolve `releases/latest`, which skips releases marked as prereleases.

## Release procedure

1. Rebase the fork onto the new upstream tag, for example `git fetch upstream --tags && git rebase v1.12.11`, resolve conflicts, and run the gates: `make check`, `make test`, `npm run test:launcher`, `npm run test:update`, `npm run test:github-actions`.
2. Merge the result into `main` on this repository.
3. Push exactly the upstream base tag, and no other upstream tags: `git push origin v1.12.11`. The release notes are generated from the previous tag reachable from the release commit. With the base tag present, the notes list only the fork's own commits. Without it they fall back to an older upstream tag and mix in upstream commits.
4. Tag the fork release on `main` and push it:
   1. `git tag -a v1.12.11-fork.1 -m v1.12.11-fork.1`
   2. `git push origin v1.12.11-fork.1`
5. Watch the workflow: `gh run watch -R itechmeat/open-code-review`. It builds six binaries, writes `sha256sum.txt`, creates the GitHub Release with build provenance, then publishes the six platform packages and the root package.
6. Verify the release:
   1. `npm view @itechmeat/open-code-review dist-tags` shows `latest` set to the new version.
   2. `npm view @itechmeat/open-code-review@1.12.11-fork.1 optionalDependencies` lists the six `@itechmeat/ocr-*` packages at the same version.
   3. `gh release view v1.12.11-fork.1 -R itechmeat/open-code-review` lists six binaries and `sha256sum.txt`.
7. A further release on the same base repeats steps 4 to 6 with `-fork.2`, and so on. After the next upstream rebase, numbering starts again at `-fork.1` on the new base.

## One-time setup for the owner

1. On npmjs.com, sign in (or sign up) as `itechmeat`, so that the `@itechmeat` scope belongs to the account. Enable two-factor authentication for the account.
2. On npmjs.com, open Access Tokens, Generate New Token, Granular:
   1. Name `ocr-fork-first-release`, expiry 7 days.
   2. Packages and scopes: read and write, limited to the `@itechmeat` scope.
   3. Tick "Bypass 2FA", because CI cannot answer a one-time password.
   4. Copy the token.
3. On GitHub, open this repository, Settings, Environments, and create the environment `npm`, which the publish job uses. Under Deployment branches and tags choose Selected branches and tags and add the tag rule `v*-fork.*`. Add the environment secret `NPM_TOKEN` with the token.
4. On GitHub, open the Actions tab and confirm "I understand my workflows, go ahead and enable them". Then disable every workflow except Release: the others expect self-hosted runners that this repository does not have, except CodeQL, which runs on GitHub-hosted runners but is not needed on the fork:

   ```bash
   for w in action-contract.yml ci.yml codeql.yml deploy-pages.yml frontend-ext.yml idea-ext.yml ocr-review.yml pages-ci.yml plugin-contract.yml translation-sync.yml vscode-ext.yml; do
     gh workflow disable "$w" -R itechmeat/open-code-review
   done
   ```

   Check `gh workflow list -R itechmeat/open-code-review` after every rebase: a workflow file newly added upstream starts out enabled.
5. Run the first release with the procedure above (`v1.12.11-fork.1`). It publishes with the token, because a trusted publisher can only be attached to a package that already exists.
6. Configure trusted publishing for all seven packages, locally with npm 11.15 or newer:

   ```bash
   npm login
   for p in open-code-review ocr-darwin-arm64 ocr-darwin-x64 ocr-linux-arm64 ocr-linux-x64 ocr-win32-arm64 ocr-win32-x64; do
     npm trust github "@itechmeat/$p" --file release.yml --repo itechmeat/open-code-review --environment npm --allow-publish -y
   done
   ```

   The web equivalent, per package: Settings, Trusted Publisher, GitHub Actions, user `itechmeat`, repository `open-code-review`, workflow `release.yml`, environment `npm`.
7. For each package on npmjs.com, open Settings, Publishing access, and choose "Require two-factor authentication and disallow tokens".
8. Delete the `NPM_TOKEN` environment secret on GitHub and revoke the token on npmjs.com.
9. The next release (`-fork.2`) proves that trusted publishing works. Its npm page shows a provenance badge, and `npm audit signatures` in a project that depends on it reports verified attestations. The workflow itself does not change between the token and the trusted-publishing setup: without the secret, the "Configure npm registry" step writes an empty token, and npm's trusted-publishing exchange supplies the real one. An E401 or E404 on that first token-less release means the trusted publisher of that package is not configured as in step 6.
10. Protect `main` with a branch ruleset (Settings, Rules, Rulesets): enable Restrict deletions, Restrict updates and Block force pushes, and add the repository admin role to the bypass list, so only the owner can update `main`, including the force push after each upstream rebase. The install one-liners run the scripts on `main`.

## Acceptance test plan

Run these checks from a scratch directory outside the repository, on Linux (x64) and on macOS (Apple Silicon). `V` is the version under test, for example `V=1.12.11-fork.1`.

### Pre-flight

1. List existing installs: `which -a ocr; npm ls -g --depth=0 2>/dev/null | grep -i open-code-review`.
2. Remove the upstream package first: `npm uninstall -g @alibaba-group/open-code-review; hash -r`. If `which -a ocr` still lists a binary from `install.sh` or Homebrew, remove it too (`sudo rm /usr/local/bin/ocr` or `brew uninstall open-code-review`).
3. `npm view @itechmeat/open-code-review dist-tags versions optionalDependencies --json` shows `latest` equal to `$V` and the six `@itechmeat/ocr-*` packages pinned to `$V`.

### Main path (platform package)

1. `npm install -g @itechmeat/open-code-review && hash -r`
2. `ocr --version` and `ocr version` print `open-code-review v$V (<commit>) <os>/<arch>`.
3. `ls "$(npm root -g)/@itechmeat/"` shows `open-code-review` and exactly one `ocr-<os>-<arch>`.
4. `which -a ocr` lists exactly one `ocr`.
5. On macOS, confirm that Gatekeeper does not block the binary.
6. From the second release on, check provenance: `mkdir -p p && cd p && npm init -y >/dev/null && npm i @itechmeat/open-code-review && npm audit signatures; cd ..`.

### Fallback path (postinstall download)

1. `npm uninstall -g @itechmeat/open-code-review`
2. `npm install -g @itechmeat/open-code-review --omit=optional --foreground-scripts`. On npm 12, which skips install scripts of global packages by default, add `--allow-scripts=@itechmeat/open-code-review`.
3. The output shows a download from `https://github.com/itechmeat/open-code-review/releases/download/v$V/...` followed by `Checksum verified.`
4. `ls "$(npm root -g)/@itechmeat/"` shows no `ocr-*` package, and `ocr version` prints the same output as on the main path.

### Real review on a tiny repository

1. Create the repository:

   ```bash
   mkdir -p demo && cd demo && git init -q
   printf 'def div(a, b):\n    return a / b\n' > m.py && git add . && git commit -qm init
   printf 'def div(a, b):\n    return a / b if b else None\n\ndef f(x):\n    return eval(x)\n' > m.py
   ```

2. `ocr review --help` works.
3. `ocr review --audience agent` reviews the working-tree change with the configured provider, reports a finding on `eval`, and exits with code 0.

### Update check

1. Install an older fork release while a newer one is published: `npm i -g @itechmeat/open-code-review@<older version>`.
2. Reset the update state: `rm -f ~/.opencodereview/last-update-check ~/.opencodereview/update-available`.
3. Run the updater directly: `node "$(npm root -g)/@itechmeat/open-code-review/scripts/update.js"; ocr version`. It queries `@itechmeat/open-code-review`, never the upstream package, and installs the newer version itself with `npm i -g`, so `ocr version` prints the newer version. `~/.opencodereview/update-available` exists only if that install failed.
4. Check the background path: repeat steps 1 and 2, run `ocr version`, wait about 30 seconds, run `ocr version` again, and confirm that it prints the newer version.

### install.sh path

1. `npm uninstall -g @itechmeat/open-code-review; hash -r`
2. `curl -fsSL https://raw.githubusercontent.com/itechmeat/open-code-review/main/install.sh | OCR_INSTALL_DIR="$HOME/.local/bin" sh` reports `installed ocr v$V`.
3. `~/.local/bin/ocr version` prints `v$V`.
4. Pinned install: `curl -fsSL https://raw.githubusercontent.com/itechmeat/open-code-review/main/install.sh | OCR_VERSION=v$V OCR_INSTALL_DIR="$HOME/.local/bin" sh`.
5. Clean up: `rm ~/.local/bin/ocr`.
