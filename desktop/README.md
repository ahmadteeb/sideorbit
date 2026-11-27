# Orbit Setup (desktop)

A fork of [iloader](https://github.com/nab138/iloader) by nab138 (MIT, see `LICENSE`), kept as a `git subtree`. It installs Orbit on an iPhone from a computer.

**Not finished.** It signs in to your Orbit server, uses that server's anisette gateway for Apple sign-in, installs an IPA you choose and manages pairing files. It does not yet have the guided wizard, driver setup, or an Orbit download.

## What Orbit changes
- `src-tauri/src/orbit/` and `src/orbit/`: the Orbit account, browser sign-in, and the anisette proxy.
- Edits to upstream files are marked `ORBIT:`.
- No auto-updater (upstream's would install iloader over it), no developer tools, no public anisette servers, our own name and icon.

The name "iloader" and its artwork belong to its author (`LICENSE-BRANDING`) and aren't used here.

## Build
```bash
bun install && bun run tauri build
```
Pull upstream changes from the repository root:
```bash
git subtree pull --prefix=desktop https://github.com/nab138/iloader.git <tag> --squash
```
