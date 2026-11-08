# AlphaRing Launcher
<img width="1005" height="757" alt="image" src="https://github.com/user-attachments/assets/f0936361-728f-45af-9073-d6347b16b4ef" />
This launcher can install either [MegaBit's AlphaRing](https://github.com/megabitt01/AlphaRing) or [Kirkland's experimental fork](https://github.com/kirklandsig/AlphaRing) for Halo: The Master Chief Collection. It can also launch unmodded MCC with anti-cheat on.

## Installation
### Windows
Grab the latest `.exe` build from Releases. The launcher stores `launcher.cfg` in `%AppData%\alpharing`, where you can specify a custom MCC install path. It normally runs without administrator rights. If Windows denies an install or uninstall file change, it requests UAC permission for that operation.
### Linux
Download the install.sh script.  Run `chmod +x ./install.sh \ ./install.sh` in the command line and the binary will install itself on your system.  It will also install protontricks and install VC redistributable as a patch for the "fatal error" issue.  To uninstall, run `./install.sh --uninstall` and it will remove the binary but leave its dependencies installed.  Currently, you'll have to create a .sh or .desktop file to start the launcher.

## Choosing a launch version

Use the dropdown to select **Vanilla MCC**, **MegaBit AlphaRing**, or **Kirkland AlphaRing (experimental)**. The **Active** line reports the DLL actually installed in MCC. Select a mod and press **Install** before **Launch** becomes available. Selecting the installed mod changes the button to **Uninstall**. Selecting Vanilla while a mod is installed offers **Uninstall mod**; Vanilla launches with anti-cheat on once the DLL is removed. Modded launches use the anti-cheat-off option.

MegaBit uses its latest non-prerelease GitHub release. Kirkland uses the newest published release with `WTSAPI32.dll`, including prereleases. The launcher checks GitHub's SHA-256 asset digest, falling back to a `.sha256` release asset where available. Installed DLLs are verified against their recorded hashes before Launch is enabled. The launcher can identify a DLL from an earlier installation when its hash matches a published release; otherwise it reports an unrecognized DLL and requires an explicit install before modded launch.

Install and Uninstall preserve the previous DLL under `alpha_ring/launcher-backups` in the MCC folder. Mod settings are left in place. The launcher no longer restores an old cached DLL automatically. The latest published Kirkland DLL may be older than that fork's current `main` branch.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.
For building on Linux, you may need to run `wails build -tags webkit2_41`.

## Launcher changes

### Version 2.0.4 — November 8, 2026

- Removed the visible "Launch version" label above the dropdown. The dropdown retains an accessible name.
- Rebuilt the Windows EXE: [AlphaRingLauncher_Windows_v2.0.4.exe](https://github.com/oraclesod/alpharing-launcher-v2/releases/download/v2.0.4/AlphaRingLauncher_Windows_v2.0.4.exe), SHA-256 `B2A33F3CEDFA1672F9B9F90AB32F13E36BDD1317EE923A5242613B593CEC39AE`. A local copy is in `../Launcher Builds/`.

Validation: Wails production build passed; the EXE reports file and product version 2.0.4. Live MCC launch remains to be checked on a game machine.

### Version 2.0.3 — November 8, 2026

- Added the launch version dropdown, Active status, explicit Install/Uninstall, and a Launch button enabled only for the installed selection.
- Added verified downloads from MegaBit and Kirkland releases, installed DLL recognition by release hash, and safe backups when switching or uninstalling.
- Kept the launcher versioned separately from either mod. The Windows EXE is [AlphaRingLauncher_Windows_v2.0.3.exe](https://github.com/oraclesod/alpharing-launcher-v2/releases/download/v2.0.3/AlphaRingLauncher_Windows_v2.0.3.exe), SHA-256 `A71981746455AA8EA36A446F5E7F3A556BBD3E9320EBACF4C84B853A2E0D0ECF`. A local copy is in `../Launcher Builds/`; the 2.0.2 and original EXEs remain preserved.

Validation: `go test ./...`, live verified DLL downloads from both GitHub releases, the frontend production build, and the Wails Windows build passed. Live MCC play, UAC on a protected installation, and controller navigation still require a game-machine test.

### Version 2.0.2 — November 8, 2026

The previous local 2.0.2 build added the following behavior:

- The launcher starts without requesting administrator rights. If Windows denies a game-file change after a play selection, a headless helper requests UAC elevation to complete those file changes. The original launcher waits for its result and then starts Steam from its own process. Canceling UAC stops the play action.
- Play actions keep the progress panel active through downloads and UAC. The menu blocks duplicate actions, and errors remain visible for six seconds.
- `wails.json` and `build/windows/info.json` set Windows file and product version 2.0.2. The config location documented above was corrected to `%AppData%\alpharing\launcher.cfg`.
- Steamworks SDK redistributable files were added locally under `assets/steamworks/` for building. Git ignores those files.

The Windows build is [`../Launcher Builds/AlphaRingLauncher_Windows_v2.0.2.exe`](../Launcher%20Builds/AlphaRingLauncher_Windows_v2.0.2.exe), SHA-256 `A118BA51C4108A41BC83101DEFD35D34C43F5E7067B3EF418921C18467EE543E`. The original [`AlphaRingLauncher_Windows.exe`](../Launcher%20Builds/AlphaRingLauncher_Windows.exe) is preserved. The new EXE is unsigned.

`go test ./...` passed with the supplied Steamworks DLL. The Wails production build completed, the required Steamworks exports were found, and the compiled headless helper returned the expected status for invalid arguments. The EXE manifest remains `asInvoker`. A live UAC and MCC/Steam play test is still needed on a protected game installation.

The EXE embeds `steam_api64.dll` for Steam Workshop access. It does not bundle a mod DLL; the selected mod is downloaded during Install. Version 2.0.3 moves Workshop setup to modded Launch and removes the old automatic backup restore path.
