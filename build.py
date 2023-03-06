import subprocess, re, sys, os, shutil, json, binascii, hashlib, gzip, stat

VER_RE = re.compile(r"v(\d\.\d+\.\d+)")

NAME = "bedrocktool"
APP_ID = "yuv.pink.bedrocktool"
TAG = subprocess.run(["git", "describe", "--exclude", "r-*", "--tags", "--always"], stdout=subprocess.PIPE).stdout.decode("utf8").split("\n")[0]
if TAG == "":
    TAG = "v0.0.0"
VER = VER_RE.match(TAG).group(1)

CI = not not os.getenv("GITLAB_CI")


with open("./utils/resourcepack-ace.go", "rb") as f:
    PACK_SUPPORT = f.read(7) == b"package"

print(f"Pack Support: {PACK_SUPPORT}")

LDFLAGS = f"-s -w -X github.com/bedrock-tool/bedrocktool/utils.Version={TAG}"

PLATFORMS = [
    ("windows", ["386", "amd64"], ".exe"),
    ("linux", ["386", "amd64", "arm", "arm64"], ""),
    #("darwin", ["amd64", "arm64"], ""),
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
        print(f"Building {platform_name} gui: {GUI}")
        SUB1 = '-gui' if GUI else ''
        exe_name = f"{NAME}{SUB1}{ext}"

        env = ["GOVCS=*:off"]
        GOFLAGS = []
        if not PACK_SUPPORT:
            GOFLAGS.append("-overlay=overlay.json")

        if GUI:
            if len(GOFLAGS):
                env.append(f"GOFLAGS={' '.join(GOFLAGS)}")
            args = [
                "fyne-cross", platform_name,
                "-app-version", VER,
                "-arch", ",".join(archs),
                "-ldflags", LDFLAGS + f" -X github.com/bedrock-tool/bedrocktool/utils.CmdName=bedrocktool-gui",
                "-name", exe_name,
                "-tags", "gui",
                "-debug"
            ]
            for e in env:
                args.extend(["-env", e])
            if platform_name == "windows":
                args.append("-console")
            if platform_name == "darwin":
                args.extend(["-app-id", APP_ID])
            args.append("./cmd/bedrocktool")
            out = subprocess.run(args)
            out.check_returncode()
        else:
            for arch in archs:
                out_path = f"./tmp/{platform_name}-{arch}/{exe_name}"
                os.makedirs(os.path.dirname(out_path), exist_ok=True)
                env.extend([f"GOOS={platform_name}", f"GOARCH={arch}"])
                env.append("CGO_ENABLED=0")
                args = [
                    "go", "build",
                    "-ldflags", LDFLAGS,
                    "-trimpath",
                    "-v",
                    "-o", out_path,
                ]
                args.extend(GOFLAGS)
                args.append("./cmd/bedrocktool")
                print(args)
                out = subprocess.run(args)
                out.check_returncode()

        for arch in archs:
            if GUI:
                exe_path = f"./fyne-cross/bin/{platform_name}-{arch}/{exe_name}"
            else:
                exe_path = f"./tmp/{platform_name}-{arch}/{exe_name}"

            with open(exe_path, "rb") as f:
                exe_data = f.read()
                sha = binascii.b2a_base64(hashlib.sha256(exe_data).digest()).decode("utf8").split("\n")[0]
            
            exe_out_path = f"./builds/{NAME}-{platform_name}-{arch}-{TAG}{SUB1}{ext}"
            with open(exe_out_path, "wb") as f:
                f.write(exe_data)
            os.chmod(exe_out_path, os.stat(exe_out_path).st_mode | stat.S_IEXEC)

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
