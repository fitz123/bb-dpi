# Control plane bootstrap

One-time setup for the pull-based control plane (Phase 1 of the
pkg-and-pull-control-plane plan — kept in operator memory per
`.gitignore`). Two steps:

1. **Dev machine** — mint the token, edit endpoint config, verify the
   semantic allowlist test passes.
2. **On each cover-site host** — drop the nginx snippet into the
   existing cover-site config, wire `@cover_404` to the cover backend,
   confirm probe-fingerprint matches.

After both steps complete, `make publish-bundle-prod` pushes
`bundle.json` to every reachable endpoint atomically (and
`make publish-bundle-test` stages a candidate on the test path — see
[§4](#4-testprod-publish-workflow)); `make publish-status` reports
per-endpoint drift for both targets; `make test-publish-bundle` guards
the allowlist invariant; `make test-cover-fingerprint` confirms the
cover disguise survives.

---

## 1. Dev machine first-time setup

Run from the project root.

### 1a. Real endpoints file (gitignored)

```
cp config/control-plane/endpoints.example.json config/control-plane/endpoints.json
$EDITOR config/control-plane/endpoints.json
```

Schema (array of objects, one per cover-site host):

| field                      | required | description                                            |
|----------------------------|----------|--------------------------------------------------------|
| `label`                    | yes      | human label for logs (e.g., `primary`, `secondary`)    |
| `url`                      | yes      | full `https://<sni>/control/bundle.json` URL (the **prod** target) |
| `url_test`                 | no       | full `https://<sni>/control/test/bundle.json` URL (the **test** target; same cover host, extra path) |
| `host_ip`                  | yes      | public IP (informational; useful for diagnostics)      |
| `sni`                      | yes      | cover-site SNI (e.g., the legitimate hostname xray's REALITY listener mimics) |
| `ssh`                      | yes\*    | `user@host` target for `scp` + `ssh mv` (\*not required if `placeholder: true`) |
| `remote_bundle_path`       | no       | where to scp the **prod** `bundle.json` on the host. Default: `/etc/bb-dpi/bundle.json` |
| `remote_bundle_path_test`  | no       | where to scp the **test** bundle on the host. Default: `/etc/bb-dpi/bundle-test.json` |
| `placeholder`              | no       | `true` when the endpoint is reserved for a future host (skipped by publish + status) |

**Test/prod path convention.** Targets are two published snapshots at
two paths on the *same* cover endpoint — no extra host, no extra SNI,
no extra token. Prod is what every client consumes by default
(`/control/bundle.json` served from `/etc/bb-dpi/bundle.json`); test is
a staging snapshot at `/control/test/bundle.json` served from
`/etc/bb-dpi/bundle-test.json`, fetched only by clients explicitly
switched with `sudo bb-vpn target test`. Both locations sit behind the
same nginx `auth_request` token gate. The bundle JSON itself is
target-agnostic (no `target` field inside), which is what makes
promotion a pure byte-copy from the test path to the prod path. Omit
`url_test` on an endpoint that doesn't serve the test location yet —
publish/status skip it rather than fail.

Seed `endpoints.json` with at least one real entry and optionally
a `placeholder: true` slot for an eventual secondary cover-site host.
The .pkg will ship both URLs baked in, so adding the secondary later is
operationally cheap.

### 1b. Mint the bearer token (gitignored)

```
openssl rand -base64 48 | tr -d '+/=\n' | cut -c1-64 > config/control-plane/token
chmod 600 config/control-plane/token
```

The token is a single 64-char URL-safe random string. Same token is used
by every endpoint and every client; it gates `/control/bundle.json` at
the nginx layer (`auth_request /__bb_auth`). Token rotation is a
"reinstall everyone" event — see the plan's threat model.

### 1c. Inspect `package-manifest.json` (already committed)

Inspect the live values — this file is the source of truth, so read it
rather than transcribing version numbers into other docs (they drift):

```
jq . config/control-plane/package-manifest.json
```

Bump this whenever you rebuild the .pkg: `bb_vpn` (the whole-package
build identity — `client/pkg-build/build.sh` derives the `.pkg` filename and
`pkgbuild --version` from it) on *every* change to what the `.pkg`
ships, not just Go sources (see [release.md §2a](release.md) for the
full list and tiers — `bb-vpn --version` is stamped from this field, so
skipping the bump ships a package reporting the old version), and
`sing_box`/`xray` when you drop in new upstream binaries. `publish-bundle` reads this and embeds it
as `bundle.min_versions`. The runtime gate (`checkBinaryVersions`)
enforces only the `sing_box` and `xray` floors — a client with older
sing-box/xray fails closed and rejects the bundle rather than rendering
bad configs; `min_versions.bb_vpn` is carried as build-identity metadata
today, **not** a runtime compatibility gate.

### 1d. Sanity-check the assembly

```
make test-publish-bundle
```

Builds a synthetic `servers.json` containing every forbidden field
(`private_key`, `ssh`, `xhttp_dest`, `sni_dest`, `relay_upstream`,
`client_render: false`), runs `publish-bundle --dry-run`, and asserts
the bundle's `.servers[]` projections contain ONLY the allowlist
fields `{name, host, public_key, short_id, xhttp_path, xhttp_sni, sni}`.

Must show: `PASS: test-publish-bundle: all assertions passed`.

### 1e. Dry-run against your real `servers.json`

```
./scripts/publish-bundle --dry-run --out /tmp/bundle.json
jq . /tmp/bundle.json | less
```

Eyeball it. Confirm:

- `issued_at` is a fresh UTC timestamp.
- `min_versions` matches `package-manifest.json`.
- `servers[]` is your `client_render != false` set with ONLY the
  allowlist fields per entry. No `private_key`, no `ssh`, no
  `xhttp_dest`.
- `skeletons.sing_box` and `skeletons.xray_xhttp` look like the
  current `config/client/*-skeleton.json` files.
- `render` matches `config/control-plane/render.json`.

---

## 2. On each cover-site host

Run as root (or via `sudo`) on the host that fronts xray's REALITY
fallback. nginx must already be serving the cover site on
`127.0.0.1:8081` (or wherever your REALITY `xhttp_dest` points).

### 2a. Prepare the bundle directory

```
mkdir -p /etc/bb-dpi
chmod 750 /etc/bb-dpi
touch /etc/bb-dpi/bundle.json
chmod 640 /etc/bb-dpi/bundle.json
chown root:<nginx-group> /etc/bb-dpi /etc/bb-dpi/bundle.json
```

`<nginx-group>` is whatever the cover-site nginx worker runs as
(typically `nginx`, `www-data`, or `http` depending on distro).

`/etc/bb-dpi/bundle-test.json` (the test target) does **not** need
pre-creation: the publish loop scps to a temp file, chowns it to match
the parent directory's owner/group, and `mv`s it into place — the first
`make publish-bundle-test` produces the file with the same
root:`<nginx-group>` 0640 ownership. Pre-`touch` it with the same modes
as `bundle.json` only if you want nginx to answer the test location
before the first test publish (otherwise an authorized GET there 404s
into `@cover_404`, which is harmless).

### 2b. Drop the location block into the cover-site nginx server

scp `config/control-plane/nginx-bundle.conf.template` to the host, then
substitute the token and write it to `/etc/nginx/snippets/` (NOT
`/etc/nginx/conf.d/` — that directory is auto-included at HTTP scope by
the stock `nginx.conf`, and bare `location {}` blocks aren't valid
there; the snippets directory is the canonical home for fragment-only
includes):

```
TOKEN=$(cat /path/to/repo/config/control-plane/token)
mkdir -p /etc/nginx/snippets
awk -v tok="$TOKEN" '{gsub("@@TOKEN@@", tok); print}' \
    nginx-bundle.conf.template > /etc/nginx/snippets/bb-dpi-bundle.conf
chmod 0600 /etc/nginx/snippets/bb-dpi-bundle.conf
chown root:root /etc/nginx/snippets/bb-dpi-bundle.conf
```

(`awk` rather than `sed` so a token containing sed-special chars like
`/` or `&` doesn't break substitution. `chmod 0600` because the
substituted file contains the bearer token — only root needs to read
it, and nginx's master reads at reload time as root before forking
workers.)

Then add `include /etc/nginx/snippets/bb-dpi-bundle.conf;` inside the
cover-site server block (the one fronting REALITY fallback on
`127.0.0.1:8081`). The included file is location-blocks-only — it
needs a parent `server { ... }` context.

The template carries **both** targets: `location = /control/bundle.json`
(prod, aliasing `/etc/bb-dpi/bundle.json`) and
`location = /control/test/bundle.json` (test, aliasing
`/etc/bb-dpi/bundle-test.json`). They share the single `/__bb_auth`
subrequest (one token, substituted once) and the same
`error_page … = @cover_404` masking, so this one include wires the test
location too — no extra per-target step. Hosts that predate the test
target just need the snippet re-substituted from the current template
and nginx reloaded.

### 2c. Wire `@cover_404`

The template ships with a placeholder `return 404;` inside the
`@cover_404` named location. **You must replace this with whatever
your cover site serves for a normal random missing URL** — otherwise
`scripts/test-cover-fingerprint` will fail loudly (and a real DPI prober
could distinguish the control endpoint from cover traffic).

Typical wirings:

- Cover backend is another nginx upstream:
  ```
  location @cover_404 {
      proxy_pass http://127.0.0.1:8081;
  }
  ```
- Cover is a static site with a custom 404 page:
  ```
  location @cover_404 {
      root /var/www/cover;
      try_files /404.html =404;
  }
  ```
- Cover backend is a remote upstream:
  ```
  location @cover_404 {
      proxy_pass http://cover-origin.internal;
      proxy_set_header Host $host;
  }
  ```

### 2d. Reload nginx + verify

```
nginx -t
systemctl reload nginx   # or service nginx reload — distro-dependent
```

Confirm the cover site still serves its normal content on the
existing paths:

```
curl -sS https://<cover-sni>/         # should match what it did before the change
```

### 2e. Probe-fingerprint test (from the dev machine, NOT the host)

```
make test-cover-fingerprint
```

Iterates `endpoints.json` (skipping `placeholder: true`), and for each
real endpoint runs:

- Baseline determinism: 5 GETs to a random `/_bbtest_<hex>` path. If
  the cover site is non-deterministic (timestamp in 404 body, dynamic
  request-id headers, etc.), the test SKIPS this endpoint with a
  warning and asks you to validate manually.
- Probe: GET `/control/bundle.json` WITHOUT `Authorization` header.
  Must hash-match the baseline.
- Adversarial probes: malformed `Authorization` (NUL byte), 16KB
  oversized token, no `Host` header. All must hash-match baseline.

`PASS: byte-identical (within tolerance)` is the green light. Any
mismatch means `@cover_404` isn't wired correctly OR the cover site is
non-deterministic in a way that would be visible to DPI.

The script's probes target the **prod** path (`/control/bundle.json`).
The test location must not fingerprint the endpoint any more than the
prod one does — after wiring it, repeat at least the no-Authorization
probe by hand against `/control/test/bundle.json` and hash-compare
against the same random-path baseline (the two locations share their
auth + masking config, so a prod PASS plus a matching manual test-path
probe is sufficient).

---

## 3. First real publish

After `test-publish-bundle` and `test-cover-fingerprint` both pass:

```
make publish-bundle-prod
make publish-status
```

`publish-bundle-prod` should report `published + verified` for each real
endpoint and `skipping (placeholder)` for any reserved-future slots.
`publish-status` should show the new `issued_at` on every endpoint.
Endpoints without `url_test` get a `test skip (no url_test)` row (not a
failure); an endpoint **with** `url_test` configured shows a failing
test row (and non-zero exit) until the first `make publish-bundle-test`
lands — publish to both targets once to go fully green.

From here, config changes reach the fleet via the staged workflow below
(or directly via `make publish-bundle-prod` when you're confident).
Clients pick a new prod bundle up on their next sync tick (15 min).

---

## 4. Test/prod publish workflow

Targets are two published snapshots at two paths on the same cover
endpoints ([§1a](#1a-real-endpoints-file-gitignored)). The staged loop
lets you validate a config change on one real client while the rest of
the fleet keeps running stable:

```
make publish-bundle-test         # 1. working tree → /control/test/bundle.json
sudo bb-vpn target test          # 2. on the designated test client (once)
sudo bb-vpn sync                 # 3. fetch the candidate now (or wait ≤15 min)
                                 # 4. validate the client behaves
make promote-bundle              # 5. byte-copy validated test bytes → prod
sudo bb-vpn target prod          # 6. flip the test client back to stable
make publish-status              # any time: issued_at/sha for BOTH targets
```

Notes:

- `bb-vpn target` with no argument prints the active target (no root);
  setting it requires root and survives reboots until reversed — same
  sentinel-file pattern as `sudo bb-vpn start/stop`. `bb-vpn status`
  also reports the active target.
- `make promote-bundle` never re-assembles: it GETs the currently
  served test bundle, aborts if test endpoints disagree on sha, then
  republishes those exact bytes to the prod path on every endpoint and
  verifies prod sha == test sha. What you validated is byte-identical
  to what ships (`issued_at` included).
- A working-tree edit made *after* step 4 cannot leak into prod via
  promote — only via a fresh `publish-bundle-test` cycle or an explicit
  `publish-bundle-prod`.
- Order matters on a fresh host: wire the nginx test location + run the
  first `publish-bundle-test` **before** flipping any client to
  `target test` — otherwise its test fetch 404s and it falls back to
  the cached bundle (graceful, but `bb-vpn status` shows a
  `last_fetch_error` until the test path serves).
- Clients without `url_test` in their baked `control-plane.json` (built
  before the field existed) ignore the test target entirely; prod
  behavior is unchanged.

---

## Threat model + ops references

- **Token rotation**: high-cost (rebuild .pkg + redeploy nginx on every
  endpoint + redistribute to all users). Avoided by keeping the .pkg
  URL out of public channels. See the plan's Threat-model section.
- **Allowlist invariance**: `scripts/test-publish-bundle` is the
  semantic guard. Add it to CI before any commit that touches
  `scripts/publish-bundle`.
- **Cover-site disguise**: `scripts/test-cover-fingerprint` is the
  empirical guard. Run after every nginx change on a cover-site host.

Full plan: kept in operator memory (gitignored per `.gitignore` PII rule).
