# Release runbook

Operator runbook for cutting a `BB-VPN-<ver>.pkg`, signing it ad-hoc,
hosting it, and rolling it out. Phase 6 of the pkg-and-pull-control-plane
plan.

The operator does **not** have an Apple Developer license. Everything is
unsigned (or ad-hoc signed). No notarization. Users see Gatekeeper
warnings on first install and first launch; the user-facing install page
walks them through the right-click → Open dance.

---

## 1. Prerequisites

One-time on the dev machine:

- Xcode Command Line Tools (`xcrun`, `pkgbuild`, `productbuild`, `lipo`,
  `codesign`, `swiftc`) — `xcode-select --install`.
- Go ≥ 1.22 — `brew install go`. `go.mod` enforces the floor.
- `jq` — `brew install jq`.
- `envsubst` (ships in `gettext`) — `brew install gettext` (needed by
  the per-user install-page recipe in [§3d](#3d-per-user-install-page)).
- Control plane bootstrap completed: `config/control-plane/endpoints.json`
  + `config/control-plane/token` exist (see
  [control-plane-bootstrap.md](control-plane-bootstrap.md)).
- `sing-box` and `xray` binaries dropped into
  `client/pkg-build/payload-binaries/` along with `geoip.dat` +
  `geosite.dat` (see `client/pkg-build/README.md`).
- `config/control-plane/package-manifest.json` versions match the
  payload binaries (`make build-pkg` aborts otherwise).

---

## 2. Build

### 2a. Bump the version — required on every change

`bb-vpn --version` is stamped from
`config/control-plane/package-manifest.json` (`bb_vpn`), and
the Makefile builds the binary *from* that field. So a code change with
no manifest bump produces a materially different binary that still
reports the old version — two builds both saying `1.0.0`,
indistinguishable in the field (you'd have to hash the Mach-O to tell
them apart). And `bb-vpn --version` is the *only* surface that carries
the build identity at all — neither `status.json` (so `bb-vpn status`)
nor BBVPN.app's bundle version exposes it — so a missed bump
leaves nothing in the field to tell two installs apart.

**Before building a `.pkg` that ships any change, bump the matching
field in `config/control-plane/package-manifest.json` following
[semver](https://semver.org/):**

- `bb_vpn` — this doubles as the **whole-package build identity**:
  `client/pkg-build/build.sh` derives both the `BB-VPN-<ver>.pkg`
  filename and `pkgbuild --version` from it. Bump it for *any* change to what the
  `.pkg` ships — `client/bb-vpn` Go sources, `BBVPN.app`/menubar,
  LaunchDaemon/Agent plists, installer scripts, the bundled UI,
  geoip/geosite data, or the baked-in `control-plane.json` token. Tier
  by impact: patch (`1.0.0 → 1.0.1`) for a fix or a token rotation;
  minor (`→ 1.1.0`) for a backward-compatible feature; major
  (`→ 2.0.0`) for a breaking bundle-schema change — and a major is a
  "deploy every client *before* publishing any bundle" event (see the
  parse-strict contract under
  [Rollout sequencing](#rollout-sequencing--read-before-publishing-any-bundle)).
- `sing_box` / `xray` — set to the exact upstream version of the binary
  you dropped into `client/pkg-build/payload-binaries/`.

The version-coupling check in §2b guarantees the manifest and the
shipped binaries *agree*; it cannot detect "code changed but version
not bumped." That discipline is yours.

### 2b. Run the build

From the project root:

```
make build-pkg
```

This runs in sequence:

1. `make build-bb-vpn-pkg` — Darwin universal `bb-vpn` (arm64+amd64 via
   `lipo`) at `build/pkg/bb-vpn`. Version from
   `package-manifest.json.bb_vpn`, baked in via `-ldflags`.
2. `client/menubar/build.sh` — Darwin universal `BBVPN.app` at
   `build/menubar/BBVPN.app`.
3. `client/pkg-build/build.sh`:
   - Version-couples bb-vpn, sing-box, xray against the manifest. Abort
     on mismatch.
   - Stages payload under `build/pkg-staging/`.
   - **Ad-hoc codesigns** `bb-vpn`, `sing-box`, `xray`, and `BBVPN.app`
     in place inside the staging tree (`codesign -s - --force` on the
     standalone Mach-O binaries; `codesign -s - --force --deep` on
     `BBVPN.app` so the bundle's `Contents/MacOS/BBVPN` is signed in
     the same invocation). No Apple identity, no notarization. The
     signatures only give each
     binary a stable code-signing identifier so the kernel's
     library-validation and TCC paths don't trip on "completely
     unsigned" binaries. Gatekeeper still treats the .pkg and .app as
     "unidentified developer" — the right-click → Open dance is still
     required on first install + first launch.
   - Runs `pkgbuild` + `productbuild`. Output:
     `client/pkg-build/dist/BB-VPN-<ver>.pkg`.

The .pkg itself is unsigned. `productbuild --sign` requires a Developer
ID Installer cert that the operator doesn't have; skipping it is the
intentional cost of zero-license distribution.

Smoke-check the bundled signatures. `build.sh` already runs
`codesign --verify --deep --strict` on the staging tree (output:
`ad-hoc signatures verified.`), so the in-pkg payload is signed.
To inspect the in-pkg signatures directly:

```
rm -rf /tmp/bb-pkg-expand
pkgutil --expand client/pkg-build/dist/BB-VPN-<ver>.pkg /tmp/bb-pkg-expand
# productbuild distribution .pkgs expand to a component subdir;
# the cpio Payload lives inside it.
cd /tmp/bb-pkg-expand/BB-VPN-component.pkg && cat Payload | gunzip | cpio -i -d
codesign -dv ./Library/Application\ Support/bb-dpi/bin/bb-vpn
codesign -dv ./Applications/BBVPN.app
```

Expected: `Signature=adhoc` for each.

### Rollout sequencing — read before publishing any bundle

**Parse-strict bundle contract**: `bundle.Parse()` uses
`DisallowUnknownFields()`, so any bb-vpn binary built before a schema
addition will REJECT a bundle that contains the new field with
"unknown field" and stop applying it (sync goes `parse_failed`; daemons
keep running on cached config but no new bundles land — including
rollback bundles). Order of operations is load-bearing:

1. Build the new `.pkg`.
2. Deploy the upgrade-install to **every client in the fleet**.
3. **Only then** publish a bundle with the new schema fields —
   `make publish-bundle-test` → validate → `make promote-bundle` (the
   staged flow below), or `make publish-bundle-prod` directly. Note the
   staged flow does NOT relax this contract: the test client must
   already run the new binary before it can parse the test bundle, and
   promote ships those same bytes to clients that must also parse them.
4. Rollback path: if a bundle goes bad, publish (to prod) a previous
   render.json that omits the new field; `omitempty` lets it disappear
   cleanly.

### Staged config rollout — test target

Config changes (server list, skeletons, render.json) don't need a
`.pkg` rebuild — they ship as bundles, and the bundle flow has two
publish targets on the same cover endpoints:

- **prod** — `/control/bundle.json`, what every client consumes by
  default.
- **test** — `/control/test/bundle.json`, a staging snapshot fetched
  only by clients explicitly switched with `sudo bb-vpn target test`.

One `.pkg` serves the whole fleet: the baked `control-plane.json`
carries each endpoint's `url_test` for **everyone**, but the target
selector is per-client runtime state (a root-owned sentinel file, the
same pattern as `manually_stopped.flag`), so prod clients never touch
the test URL. Switching is `sudo bb-vpn target test|prod` — no
reinstall, survives reboots; `bb-vpn target` (no arg) and `bb-vpn
status` report the active target.

The loop, end to end:

```
make publish-bundle-test      # working tree → test path on every endpoint
sudo bb-vpn target test       # on the designated test client (once)
sudo bb-vpn sync              # pick up the candidate now
# …validate…
make promote-bundle           # republish the exact validated test bytes to prod
sudo bb-vpn target prod       # test client back on stable
make publish-status           # both targets' issued_at/sha — confirm equality
```

`promote-bundle` is a byte-copy, not a re-assembly — prod gets exactly
the sha you validated. Full workflow detail + nginx test-location
wiring: [control-plane-bootstrap.md §4](control-plane-bootstrap.md).
(`make publish-bundle` with no target is now a stub that errors,
pointing at the three explicit targets.)

### Bundled metacubexd UI

The .pkg now ships the metacubexd dashboard inside its payload at
`/Library/Application Support/bb-dpi/ui/` (served by sing-box's
clash_api at `http://127.0.0.1:9090/`). The bundled snapshot is
**not** vendored — operators must drop a fresh
`payload-binaries/ui/` (extracted from the metacubexd `gh-pages`
zip) into `client/pkg-build/` before `make build-pkg`, or the build
hard-fails. See [`client/pkg-build/README.md`](../client/pkg-build/README.md)
§Pre-flight, bullet 2 (`ui/`) for the exact refresh recipe.

### bb-vpn symlink — /usr/local/bin

Postinstall now creates the convenience symlink at
`/usr/local/bin/bb-vpn → /Library/Application Support/bb-dpi/bin/bb-vpn`
(removing the older per-user `~/.local/bin/bb-vpn` link). `/usr/local/bin`
is on macOS's default `$PATH` and on sudo's `secure_path`, so both
`bb-vpn …` and `sudo bb-vpn …` resolve without absolute-path dancing.
Old installs get the legacy `~/.local/bin/bb-vpn` cleaned up by the
uninstaller.

---

## 3. Host the .pkg + install page

The .pkg and the user-facing install page get served from a separate
nginx `location` on the cover-site backend (the same nginx that already
fronts xray's REALITY fallback on `127.0.0.1:8081`).

**Why a separate location, not under `/control/bundle.json`'s
`auth_request` umbrella**: browsers can't add `Authorization` headers on
plain navigation, so a token-gated download URL would just hand every
user a 401. The protection here is path-obscurity:

```
https://<cover-sni>/d/<32-hex>/BB-VPN-<ver>.pkg
https://<cover-sni>/d/<32-hex>/install-<short-name>.html
```

The 32-hex segment is per-cohort random. Anyone with the URL has the
.pkg (which itself contains the bearer token in
`control-plane.json`); rotation cost is documented in
[§5](#5-token-rotation).

### 3a. Mint a random path

```
PATH_HEX=$(openssl rand -hex 16)
echo "$PATH_HEX"
```

Record this somewhere out-of-band (1Password, operator notes). Treat it
as a secret of the same blast-radius as the token: leaked path = leaked
.pkg = leaked token.

### 3b. nginx location

On each cover-site host that should serve downloads, drop this snippet
into the same server block where `/control/bundle.json` lives (NOT
gated by `auth_request`). Replace `<PATH_HEX>` with the hex string
from §3a before reload — same pattern as the control-plane bootstrap
`@@TOKEN@@` substitution:

```
PATH_HEX=<32-hex from §3a>
awk -v ph="$PATH_HEX" '{gsub("<PATH_HEX>", ph); print}' \
    snippet-template.conf > /etc/nginx/snippets/bb-dpi-downloads.conf
```

Template (`snippet-template.conf`):

```
# /etc/nginx/snippets/bb-dpi-downloads.conf
location ^~ /d/<PATH_HEX>/ {
    alias /var/www/bb-dpi-downloads/;
    autoindex off;
    add_header Cache-Control "no-store";
    # Same cover fallthrough as /control/bundle.json — any miss/error
    # leaks to the cover site's natural 404 so the directory itself
    # isn't fingerprintable. Mirror the wide status list from
    # nginx-bundle.conf.template so malformed-Authorization edge cases
    # (400/413/414) and internal 5xx also funnel through @cover_404.
    error_page 400 401 403 404 405 408 413 414 444 500 502 503 504 = @cover_404;
}
```

`^~` prevents the location from also matching adjacent regex
`location ~` blocks (e.g., a `.html$` PHP handler on the cover site).
`@cover_404` is already wired during control-plane bootstrap.

Then in the parent server block:

```
include /etc/nginx/snippets/bb-dpi-downloads.conf;
```

Reload:

```
nginx -t
systemctl reload nginx
```

### 3c. Drop the files

`<nginx-group>` is whatever the cover-site nginx worker runs as
(typically `www-data` on Debian/Ubuntu, `nginx` on RHEL/Alpine,
`http` on Arch). Discover the local value with:

```
ps -o group= -p "$(pgrep -f 'nginx: worker' | head -1)"
```

Then drop the .pkg into the download dir:

```
sudo mkdir -p /var/www/bb-dpi-downloads
sudo chown -R root:<nginx-group> /var/www/bb-dpi-downloads
sudo chmod 0750 /var/www/bb-dpi-downloads
sudo cp BB-VPN-<ver>.pkg /var/www/bb-dpi-downloads/
sudo chmod 0640 /var/www/bb-dpi-downloads/BB-VPN-<ver>.pkg
```

(Repeat the install-page step below per user.)

### 3d. Per-user install page

`client/pkg-build/install-page-template.html` is the template. Fill the
placeholders with `envsubst`:

```
export PKG_URL="https://<cover-sni>/d/<PATH_HEX>/BB-VPN-<ver>.pkg"
export PKG_NAME="BB-VPN-<ver>.pkg"
export ENROLL_URI=$(./scripts/xray-users enroll-url "<Device Name>")
export USER_NAME="<First Last>"
envsubst < client/pkg-build/install-page-template.html \
    > /tmp/install-<short-name>.html
scp /tmp/install-<short-name>.html \
    cover-host:/var/www/bb-dpi-downloads/install-<short-name>.html
ssh cover-host 'sudo chmod 0640 /var/www/bb-dpi-downloads/install-*.html && \
    sudo chown root:<nginx-group> /var/www/bb-dpi-downloads/install-*.html'
```

(Use a non-guessable `<short-name>` — first name + 4 random hex is
plenty.) The DM the user receives is just the URL of this HTML page.

---

## 4. Upgrade flow

`.pkg` is fully self-contained. The user downloads `BB-VPN-1.0.1.pkg`,
right-clicks → Open, runs through the install dialog. No uninstall
required.

What postinstall does on top of an existing install:

- Overwrites every payload file (bb-vpn binary, sing-box, xray,
  BBVPN.app, plists, geoip.dat/geosite.dat).
- Re-runs `lsregister` for BBVPN.app so `bb-vpn://` URL handler resolves
  to the new binary.
- `launchctl bootout` + `launchctl bootstrap` for the sync daemon and
  the menubar LaunchAgent (the bb-vpn binary they exec resolves to the
  new copy at next spawn).
- Calls `bb-vpn start` (guarded by `manually_stopped.flag` absence + 
  `identity.json` presence). For an already-installed user with an
  active identity, this kicks the daemons fresh.
- **Does NOT clobber** `manually_stopped.flag` on reinstall. A user who
  stopped the VPN stays stopped through an upgrade.
- **Does NOT clobber** `identity.json` (the user's UUID + enrollment).
  Enrollment survives upgrades. The same .pkg can be installed onto a
  pristine Mac (where postinstall skips `bb-vpn start` because
  `identity.json` is absent — user clicks the enroll link in the
  install page to enroll).

For an already-enrolled user (the upgrade case), postinstall calls
`bb-vpn start` immediately after bootstrapping the sync daemon, so
sing-box and xray are re-bootstrapped within seconds — not on the
next 15-min sync tick. The first sync tick still runs at install time
(`RunAtLoad=true` on the sync daemon) and the daemons go through
pre-restart validation (sing-box `check`, xray `-test`). On a pristine
install (no `identity.json`), postinstall skips `bb-vpn start`; the
user enrolls via the install-page link and the first sync triggers
the daemons.

To fully remove an existing install before reinstalling (rarely
needed — the .pkg's postinstall is reinstall-safe), the uninstaller
ships in the same payload as `bb-vpn` and lives at
`/Library/Application Support/bb-dpi/bin/bb-vpn-uninstall` (see
[§6 verification](#6-verification) for the one-liner).

`control-plane.json` (endpoint URLs + bearer token) is **baked into the
.pkg payload** at build time. On reinstall, the new copy replaces the
old — which means token rotation is a "reinstall everyone" event.

---

## 5. Token rotation

Token rotation is high-cost and not routine. Trigger only on suspected
compromise of the .pkg (URL leak with enrollment data, lost laptop with
active install, etc.). Roll out plan:

| step | action                                                           | est. time |
|------|------------------------------------------------------------------|-----------|
| 1    | Mint a new token: `openssl rand -base64 48 \| tr -d '+/=\n' \| cut -c1-64 > config/control-plane/token && chmod 600 config/control-plane/token` (the `chmod` re-asserts the 0600 mode from control-plane bootstrap §1b, in case the file was deleted between rotations and a fresh umask write left it world-readable) | <1 min |
| 2    | Redeploy nginx snippet on every cover-site host: re-substitute `@@TOKEN@@`, reload nginx (see [control-plane-bootstrap.md](control-plane-bootstrap.md) §2) | ~5 min × n hosts |
| 3    | `make publish-bundle-prod` — push the new bundle.json. The push is scp/ssh (token-independent); the curl verify reads `config/control-plane/token` — already the **new** token from step 1 — and checks it against nginx, already reloaded in step 2. So step 3 runs entirely on the new token; there is no old token still live by this point. (If you'd rather verify before the auth cutover, swap the order of steps 1-2 and 3.) The test target needs no separate rotation step: both `/control/` locations share the one `/__bb_auth` gate, so step 2's reload re-keys it too; any stale test bundle just sits behind the new token until the next `publish-bundle-test`. | <1 min |
| 4    | `make publish-status` — confirm every endpoint serves the new bundle | <1 min |
| 5    | **Bump `package-manifest.json.bb_vpn`** (a patch bump is fine for a token-only rebuild), then `make build-pkg` — required, not optional. `client/pkg-build/build.sh` derives both the `.pkg` filename and `pkgbuild --version` from this field, so a token rebuild *without* a bump yields a second same-version `BB-VPN-<ver>.pkg` carrying a *different* `control-plane.json` token — indistinguishable from the old one. (`min_versions.bb_vpn` is build-identity metadata, not a runtime gate; see step 9 for what rotation actually does to un-migrated clients.) | ~3 min |
| 6    | Mint a fresh download path (`openssl rand -hex 16`), update nginx `/d/` snippet on every cover-site host, reload nginx, drop the new .pkg in the new path | ~5 min × n hosts |
| 7    | Regenerate per-user install pages with the new `PKG_URL`, host them | ~1 min × n users |
| 8    | Slack DM every user: new install URL + deadline (24-48h). Frame it honestly per the timing note below — "install the new build to keep receiving config updates, and before the old path/servers are retired," not "your install stops working at the deadline" (it keeps tunneling on cached config) | ~15 min × n users (interactive) |
| 9    | After deadline: a straggler still on the old token gets 401 from `/control/bundle.json` every tick. bb-vpn treats that as a degraded-but-successful tick and **falls back to its cached bundle** (`fetch_failed_using_cached`, sync.go) — it keeps tunneling on the stale config and is **not** auto-disabled (`runtime_blackhole` only fires on a failed post-restart smoke test, never on auth failure; cphttp collapses 401 into a generic fetch error). It stops working only once the old servers/path are retired, or you sweep it manually (run the uninstaller, or have them install the new .pkg). A hard 401→force-out would require typed auth-failure handling in sync — not current behavior. | open-ended |

**Timing — the auth cutover is immediate, not at the deadline.** The
moment step 2's nginx reload lands, every not-yet-upgraded client's
*old* baked token starts returning 401 from `/control/bundle.json`.
Per step 9 those clients keep tunneling on their cached bundle, but they
are **control-plane-deaf for the entire adoption window** — no new
bundle, rollback, server-list change, or emergency config push reaches
them until they install the new `.pkg`. The single-token model has no
built-in grace period. If you need stragglers to stay reachable during
the window, serve the *old and new* tokens from nginx until the deadline
and remove the old one after adoption — a code/infra change this runbook
doesn't currently script. Until then, don't start a rotation you can't
finish quickly, and avoid rotating during an active incident when you
might need to push a rollback to un-upgraded clients.

Total dev-machine time for a 7-user fleet on 1 cover-site host:
~30-45 min active, plus the user-driven adoption tail (1-3 days).

---

## 6. Verification

After a fresh build + host:

1. From a **clean Mac** (or wipe an existing install on a test machine
   by running the shipped uninstaller):
   ```
   sudo "/Library/Application Support/bb-dpi/bin/bb-vpn-uninstall"
   ```
   The uninstaller boots out every daemon + LaunchAgent, removes both
   `/Library/Application Support/bb-dpi/` and `/Applications/BBVPN.app`,
   and clears the LaunchDaemon plists. On a pristine Mac there's
   nothing to wipe — skip to step 2.
2. In a browser, open the per-user install page URL. Click the download
   button.
3. In Finder, right-click `BB-VPN-<ver>.pkg` → Open. Gatekeeper warning
   dialog → "Open" → password prompt → install completes.
4. Watch for the menubar icon. Postinstall bootstraps the BBVPN
   LaunchAgent so the app auto-launches; the icon (grey on a pristine
   install — not yet enrolled) should appear within a few seconds.
   If it doesn't, open `/Applications/BBVPN.app` once via right-click
   → Open to clear Gatekeeper's first-launch prompt.
5. In the install page, click the `bb-vpn://enroll?uuid=...` link.
   BBVPN.app receives the URL via `LSGetApplicationForURL`, shells out
   to `bb-vpn enroll`. Menubar icon turns yellow (first sync in flight)
   then green (synced, daemons up).
6. Verify exit:
   - The menubar's `exit server: <name> (<host>)` row should show the
     server that urltest currently has picked (live from sing-box's
     clash-api on the menubar's 5s tick).
   - Click "Open dashboard…" to confirm the bundled metacubexd UI loads
     at `http://127.0.0.1:9090/` and reflects the same `auto` pick.
   - Cross-check the public IP if you want a belt-and-braces sanity:
     `curl -fsS https://api.ipify.org` from the Mac should return the
     selected exit's IP (or an upstream's, if a relay chain is in play).

If you need to inspect launchd state directly during verification,
remember system-domain bootout requires root:

```
sudo launchctl bootout system/com.bb-dpi.bb-vpn-sync
sudo launchctl bootout system/com.sing-box-vpn
sudo launchctl bootout system/com.xray-xhttp
```

Subsequent launches don't need right-click → Open. Gatekeeper remembers
the user's first-time approval.

---

## Don'ts

- **Don't** post the install page URL in any public channel. The URL is
  the only barrier between a stranger and the bearer token baked into
  the .pkg.
- **Don't** `productbuild --sign` with an arbitrary identity. Without a
  Developer ID Installer cert, the result is worse than unsigned (it
  positively asserts an unknown signer, which Gatekeeper rejects more
  aggressively than no signature at all).
- **Don't** put the .pkg behind Cloudflare or another TLS-terminating
  CDN — the cover-site SNI camouflage relies on the cover host serving
  its own LE cert; a CDN would serve its own cert and break the
  REALITY camouflage at the same time.
- **Don't** reuse a download path across rotations. After token rotation
  (§5), retire the old `/d/<PATH_HEX>/` location entirely — leaving it
  alive lets an attacker who scraped the old URL keep downloading old
  .pkgs (which contain the old, still-cached bundle).
