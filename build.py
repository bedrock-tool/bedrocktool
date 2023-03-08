import subprocess, re, sys, os, shutil, json, binascii, hashlib, gzip

VER_RE = re.compile(r"v(\d\.\d+\.\d+)(?:-(\d+)-(\w))?")

NAME = "bedrocktool"
APP_ID = "yuv.pink.bedrocktool"
GIT_TAG = subprocess.run(["git", "describe", "--exclude", "r*", "--tags", "--always"], stdout=subprocess.PIPE).stdout.decode("utf8").split("\n")[0]
if GIT_TAG == "":
    GIT_TAG = "v0.0.0"
VER_MATCH = VER_RE.match(GIT_TAG)
VER = VER_MATCH.group(1)
PATCH = VER_MATCH.group(2) or "0"
TAG = f"{VER}-{PATCH}"

print(f"VER: {VER}")
print(f"TAG: {TAG}")

GITHUB_OUTPUT = os.getenv("GITHUB_OUTPUT")

if GITHUB_OUTPUT:
    with open(GITHUB_OUTPUT, "a") as f:
        f.write(f"release_tag=r{VER}\n")


with open("./subcommands/resourcepack-d/resourcepack-d.go", "rb") as f:
    PACK_SUPPORT = f.read(100).count(b"package ") > 0
print(f"Pack Support: {PACK_SUPPORT}")

LDFLAGS = f"-s -w -X github.com/bedrock-tool/bedrocktool/utils.Version={TAG}"

PLATFORMS = [
    ("windows", ["amd64"], ".exe"),
    ("linux", ["amd64"], ""),
    #("darwin", ["amd64", "arm64"], ""),
    ("android", ["arm64"], ".apk")
]

platform_filter = ""
arch_filter = ""
if len(sys.argv) > 1:
    platform_filter = sys.argv[1]

if len(sys.argv) > 2:
    arch_filter = sys.argv[2]

if os.path.exists("./tmp"):
    shutil.rmtree("./tmp")
os.mkdir("./tmp")

if os.path.exists("./builds"):
    shutil.rmtree("./builds")
os.mkdir("./builds")

if os.path.exists("./updates"):
    shutil.rmtree("./updates")
os.mkdir("./updates")

for (platform_name, archs, ext) in PLATFORMS:
    if platform_filter and platform_filter != platform_name:
        continue
    archs = [a for a in archs if arch_filter == "" or a == arch_filter]
    if len(archs) == 0:
        continue
    for GUI in [False, True]:
        if platform_name in ["android"] and not GUI:
            continue

        print(f"Building {platform_name} gui: {GUI}")
        SUB1 = '-gui' if GUI else ''
        name = f"{NAME}{SUB1}"

        tags = []
        if PACK_SUPPORT:
            tags.append("packs")

        env = ["GOVCS=*:off"]

        if GUI:
            args = [
                "fyne-cross", platform_name,
                "-app-version", VER,
                "-arch", ",".join(archs),
                "-ldflags", LDFLAGS + f" -X github.com/bedrock-tool/bedrocktool/utils.CmdName=bedrocktool-gui",
                "-name", name,
                "-tags", ",".join(["gui"] + tags),
                "-debug"
            ]
            for e in env:
                args.extend(["-env", e])
            if platform_name == "windows":
                args.append("-console")
            if platform_name in ["android"]:
                args.extend(["-app-id", APP_ID])
            args.append("./cmd/bedrocktool")
            out = subprocess.run(args)
            out.check_returncode()
        else:
            for arch in archs:
                out_path = f"./tmp/{platform_name}-{arch}/{name}{ext}"
                os.makedirs(os.path.dirname(out_path), exist_ok=True)
                env.extend([f"GOOS={platform_name}", f"GOARCH={arch}"])
                env.append("CGO_ENABLED=0")
                args = [
                    "go", "build",
                    "-ldflags", LDFLAGS,
                    "-trimpath",
                    "-tags", ",".join(tags),
                    "-v",
                    "-o", out_path,
                ]
                args.append("./cmd/bedrocktool")
                print(args)
                out = subprocess.run(args)
                out.check_returncode()

        for arch in archs:
            exe_out_path = f"./builds/{NAME}-{platform_name}-{arch}-{TAG}{SUB1}{ext}"

            if platform_name == "android":
                apk_path = f"./fyne-cross/dist/android-{arch}/{name}{ext}"
                #shutil.copy(apk_path, exe_out_path) # dont upload builds yet, its not usable lol
            else:
                if GUI:
                    exe_path = f"./fyne-cross/bin/{platform_name}-{arch}/{name}"
                else:
                    exe_path = f"./tmp/{platform_name}-{arch}/{name}{ext}"

                with open(exe_path, "rb") as f:
                    exe_data = f.read()
                    sha = binascii.b2a_base64(hashlib.sha256(exe_data).digest()).decode("utf8").split("\n")[0]
                shutil.copy(exe_path, exe_out_path)

                updates_dir = f"./updates/{NAME}{SUB1}"
                os.makedirs(updates_dir, exist_ok=True)
                with open(f"{updates_dir}/{platform_name}-{arch}.json", "w") as f:
                    f.write(json.dumps({
                        "Version": TAG,
                        "Sha256": sha,
                    }, indent=2))
                
                os.makedirs(f"{updates_dir}/{TAG}", exist_ok=True)
                with gzip.open(f"{updates_dir}/{TAG}/{platform_name}-{arch}.gz", "wb") as f:
                    f.write(exe_data)

                if not GUI:
                    os.remove(exe_path)
