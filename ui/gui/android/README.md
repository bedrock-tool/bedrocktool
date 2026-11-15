# Android GUI

The Android app is the same program as the PC GUI, built with `gogio` instead of `go build`.
Most of the GUI is shared. Where Android needs something different, the
difference sits in files that only one platform compiles.

## Keeping the Android and PC GUIs apart

| File | Built for | Changing it affects |
|---|---|---|
| `foo.go` (no suffix) | both | both |
| `foo_android.go` | Android only (Go applies the `_android` suffix itself) | Android only |
| `foo_desktop.go` starting with `//go:build !android` | Windows, Linux, macOS | PC only |
| anything in `ui/gui/android/` | Android only | Android only |

Every platform hook comes as a pair with the same signature: one in
`_android.go`, one in `_desktop.go`. The shared code calls it without
knowing which one it gets. The hooks so far:

| Shared code calls | PC (`_desktop.go`) | Android (`_android.go`) |
|---|---|---|
| `platformInit()` in `cmd/bedrocktool/main.go` | nothing | saves the last run's crash and catches this run's |
| `g.platformStart()` in `ui/gui/gui.go` | nothing | shows the crash report popup after a crash |
| `l.platformButtons()` in `ui/gui/logger.go` | nothing | "Report a problem" in the logs |
| `entry.actions.Layout()` in `ui/gui/pages/worlds/worlds.go` | nothing | "Open in Minecraft" and "Export" on saved worlds |
| `g.platformViewEvent()` in `ui/gui/gui.go` | nothing | keeps the window's View, which Export's dialog needs |
| `p.platformCards()` in `ui/gui/pages/settings/settings.go` | nothing | "Captures and saved worlds" on the start screen |
| `Router.OnSession` in `ui/gui/pages/router.go` (set by `platformStart`) | not set | runs the foreground service while a session runs |

To change a screen for one platform only:

1. If a hook already covers the spot, edit that platform's file.
2. If not, add a pair: a call in the shared file, and a `_desktop.go` and an
   `_android.go` version of the function. Keep the desktop version to what
   the shared code did before, so the PC build doesn't change.
3. Put Android-only logic (JNI, Android files, reports) in this package and
   call it only from `_android.go` files. The PC build never compiles it.

To keep merges from upstream easy, keep changes to shared files to the
one-line hook calls, and put everything else in platform files.

## Strings

Android-only text lives in `locale/en.yaml` here, not in the shared
`locale/*.yaml`. It's loaded with `locale.AddMessages` and looked up with
`android.T("key", nil)`. Translations go next to it as `de.yaml`, `fr.yaml`
and so on; a missing one falls back to English. `strings_test.go` checks that
every `T("...")` key in a `*_android.go` file exists and every key is used.
It runs on the PC too: `go test ./ui/gui/android/`.

## Java

- **`BedrocktoolAndroid.java`:** the calls Go can't make alone, which are
  copying a world into Downloads with MediaStore, the Export dialog, handing
  a world to Minecraft, and starting the foreground service.
- **`SessionService.java`:** a foreground service, started when a session
  starts and stopped when it ends (`session_android.go`). While it runs,
  Android shows a notification and leaves the app running in the background,
  where it is while the user plays in Minecraft. It holds a partial wake lock
  and a Wi-Fi lock so the connection survives the screen turning off. Its
  type is `specialUse`; Google Play would want that justified, a sideloaded
  APK doesn't care.
- **`WorldProvider.java`:** a read-only file provider. Minecraft only imports
  links whose path ends in `.mcworld`, and MediaStore's links end in a
  number, so "Open in Minecraft" gives it
  `content://yuv.pink.bedrocktool.worlds/worlds/…/World.mcworld` instead.
  The provider is declared in `manifest-application.xml`, which the patched
  gogio in `gio-cmd/` adds to the manifest (`-androidapp`). Stock gogio
  can't.

gogio puts `bedrocktool_android.jar` into the APK. After editing a `.java`
file, rebuild the jar and commit both:

```sh
ANDROID_HOME=~/AppData/Local/Android/Sdk sh ui/gui/android/build_jar.sh
```

## Building the APK

Needs the Android SDK and NDK, a JDK, and the patched gogio in the `gio-cmd`
submodule (bedrock-tool/gio-cmd, branch `bedrocktool-android`; run
`git submodule update --init gio-cmd` first):

```sh
export ANDROID_HOME=~/AppData/Local/Android/Sdk
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<version>
(cd gio-cmd && GOWORK=off go build -o ../gogio.exe ./gogio)
./gogio.exe -target android -arch arm64 -appid yuv.pink.bedrocktool -minsdk 21 \
  -icon icon.png -tags gui -androidapp ./ui/gui/android/manifest-application.xml \
  -androidperms android.permission.FOREGROUND_SERVICE,android.permission.FOREGROUND_SERVICE_SPECIAL_USE,android.permission.POST_NOTIFICATIONS,android.permission.WAKE_LOCK,android.permission.REQUEST_INSTALL_PACKAGES \
  -signkey ~/.android/debug.keystore -signpass android \
  -ldflags "-s -w -checklinkname=0 -X github.com/bedrock-tool/bedrocktool/utils.Version=<version>" \
  -o bedrocktool.apk ./cmd/bedrocktool
```

- **`-checklinkname=0`:** pion's `github.com/wlynxg/anet` (WebRTC, for
  NetherNet) links to `net.zoneCache`, which Go 1.23 and newer refuse by
  default.
- **`-signkey`:** without it gogio signs each build with a new random key,
  and Android won't update an installed app signed with a different key.
  The standard debug keystore keeps the key the same between local builds.
  Releases are signed with the release key instead (see "Release signing").
- **`build.go`:** `go run build.go android gui` does the same, including
  building the patched gogio. It also empties `builds/` first.
- **Minecraft already running:** if Minecraft gets a world while it's
  running in the background as a separate copy, for example from a file
  manager, it aborts (`BootstrapConfig::initialize() should only be called
  once`). "Open in Minecraft" launches with `CLEAR_TOP|SINGLE_TOP`, so a
  running Minecraft receives the world instead (tested on Minecraft 1.26.62).
- **Export:** it uses `BedrocktoolAndroid.exportFile`, not Gio's
  `explorer.CreateFile`. Gio's asks Android for a `.mcworld` MIME type, gets
  none, and its save request then matches no app ("No apps can perform this
  action").

## Captures and saved worlds

The app's folder (`Android/data/yuv.pink.bedrocktool/files`) can't be
opened by file managers on Android 11 and newer, so the start screen has a
"Captures and saved worlds" card (`filespopup_android.go`). It lists
`captures/*.pcap2` and `worlds/**/*.mcworld`, newest first:

- **Replay** and **Replay merged** start the worlds mode on the capture,
  with its saved settings and "Merge all worlds into one" off or on
  (`replayCapture` in `ui/gui/pages/settings/settings_android.go`).
- **Open in Minecraft** and **Export** work as on the finished screen.
  Export on a capture saves the `.pcap2`, to replay it on a PC.
- **Delete** asks first. Exported copies and copies in Downloads stay.

## Updates

`utils/updater` is split like the GUI: `updater_desktop.go` replaces the
PC executable and restarts it; `updater_android.go` can't, since an app
can't replace itself on Android. It:

- looks at bedrock-tool/bedrocktool's releases, newest first, for one with
  `bedrocktool-gui-android-arm64-<version>.apk` (build.go's name) and a
  SHA-256 digest;
- downloads it to `files/update/`, checks the digest, and opens Android's
  installer on it through `WorldProvider` (`update_android.go`). The user
  confirms there, and Android closes the app and installs the new one. The
  first time, Android sends the user to "Install unknown apps" instead.
- Until a release has an APK, the check finds nothing and stays quiet.

An update only installs over the app if it's signed with the same key, so
every release is signed with the one release key (see "Release signing").

## Release signing

CI signs the APK with the release key from the repository's Actions
secrets, never the debug keystore:

- `ANDROID_KEYSTORE_BASE64`: the keystore file, base64-encoded. The
  workflow decodes it to a temporary file and sets `ANDROID_KEYSTORE`.
- `ANDROID_KEYSTORE_PASSWORD`: its password (the key's must be the same).

`build.go` signs with `ANDROID_KEYSTORE` and `ANDROID_KEYSTORE_PASSWORD`
when they're set, and on CI it skips the APK when they aren't, so no
release ever goes out signed with another key. To make the key once:

```sh
keytool -genkeypair -keystore release.keystore -alias bedrocktool   -keyalg RSA -keysize 4096 -validity 10000 -dname "CN=bedrocktool"
base64 -w0 release.keystore   # paste into ANDROID_KEYSTORE_BASE64
```

Keep `release.keystore` and its password somewhere safe outside the
repository: if the key is lost, installed apps can't update to new
releases and have to be uninstalled first.

## Bug reports

"Copy and open a GitHub issue" opens `IssuesURL` in `report_android.go`
(bedrock-tool/bedrocktool).
