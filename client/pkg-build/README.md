# client/pkg-build

Bash-driven .pkg builder for BB-VPN. Produces an unsigned distribution
`.pkg` ready for `right-click → Open` install on operator-trusted Macs.

## Pre-flight (operator, one-time per release)

1. Update `config/control-plane/package-manifest.json` with the
   sing-box / xray / bb-vpn versions you intend to ship.
2. Drop the bundled binaries into `client/pkg-build/payload-binaries/`:
   - `sing-box` — Darwin universal (or whatever arch you ship);
     download from <https://github.com/SagerNet/sing-box/releases>.
   - `xray` — Darwin universal; download from
     <https://github.com/XTLS/Xray-core/releases>.

   The directory is gitignored — these are not vendored.
3. `make build-pkg`

The build script will:
- run the **version-coupling check** (bb-vpn `--version`, `sing-box
  version`, `xray version`) — must all match the manifest exactly
- stage payload + LaunchDaemon plists under `build/pkg-staging/`
- run `pkgbuild` (component) + `productbuild` (distribution)
- emit `client/pkg-build/dist/BB-VPN-<ver>.pkg`

## Postinstall behaviour

The .pkg `postinstall` runs as root and (in order):

1. Sets ownership/perms on the payload (`bin/` mode 0755, plists
   root:wheel 0644, `inbox/` sticky+world-writable 1733).
2. Creates `bundles/configs/staging/inbox` under
   `/Library/Application Support/bb-dpi/` and `/Library/Logs/bb-dpi/`.
3. Detects the console user and substitutes `BB_VPN_HOME` in the
   bb-vpn-sync plist with `/Users/<console-user>`.
4. Creates a `~/.local/bin/bb-vpn` symlink for the console user (best
   effort — failures are logged but non-fatal).
5. Bootouts `com.sing-box-vpn` + `com.xray-xhttp` — load-bearing
   order, before bb-vpn-sync is bootstrapped.
6. Bootouts + bootstraps `com.bb-dpi.bb-vpn-sync` — handles both
   fresh install and reinstall-over-existing. RunAtLoad=true on the
   plist means the daemon's first sync fires within seconds, which
   re-bootstraps sing-box/xray from the now-fresh plists.

## Uninstall

```
sudo "/Library/Application Support/bb-dpi/bin/bb-vpn-uninstall"
```

Removes daemons, plists, `/Library/Application Support/bb-dpi/`,
`/Library/Logs/bb-dpi/`, and the console-user terminal symlink.
Brew-installed sing-box/xray (if any) are untouched.
