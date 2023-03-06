import subprocess, re, sys, os, shutil, json, binascii, hashlib, gzip

VER_RE = re.compile(r"v(\d\.\d+\.\d+)")

NAME = "bedrocktool"
APP_ID = "yuv.pink.bedrocktool"
TAG = subprocess.run(["git", "describe", "--exclude", "r-*", "--tags", "--always"], stdout=subprocess.PIPE).stdout.decode("utf8").split("\n")[0]
VER = VER_RE.match(TAG).group(1)


with open("./utils/resourcepack-ace.go") as f:
    PACK_SUPPORT = f.read(7) == "package"

print(f"Pack Support: {PACK_SUPPORT}")

LDFLAGS = f"-s -w -X github.com/bedrock-tool/bedrocktool/utils.Version={TAG}"

PLATFORMS = [
    ("windows", ["386", "amd64"], ".exe"),
    ("linux", ["386", "amd64", "arm", "arm64"], ""),
    ("darwin", ["amd64", "arm64"], ""),
]

platform_filter = ""
if len(sys.argv) > 1:
    platform_filter = sys.argv[1]

if os.path.exists("./builds"):
    shutil.rmtree("./builds")
os.mkdir("./builds")

if os.path.exists("./updates"):
    shutil.rmtree("./updates")
os.mkdir("./updates")
os.mkdir(f"./updates/{TAG}")

for (platform_name, archs, ext) in PLATFORMS:
    if platform_filter and platform_filter != platform_name:
        continue
    print(f"Building {platform_name}")
    exe_name = f"{NAME}{ext}"
    args = [
        "fyne-cross", platform_name,
        "-app-version", VER,
        "-arch", ",".join(archs),
        "-ldflags", LDFLAGS,
        "-name", exe_name,
        "-env", "GOVCS=off"
    ]
    if platform_name == "windows":
        args.append("-console")
    if platform_name == "darwin":
        args.extend(["-app-id", APP_ID])

    args.append("./cmd/bedrocktool")
    out = subprocess.run(args)
    out.check_returncode()

    for arch in archs:
        exe_path = f"./fyne-cross/bin/{platform_name}-{arch}/{exe_name}"
        with open(exe_path, "rb") as f:
            exe_data = f.read()
            sha = binascii.b2a_base64(hashlib.sha256(exe_data).digest()).decode("utf8").split("\n")[0]

        with open(f"./updates/{platform_name}-{arch}.json", "w") as f:
            f.write(json.dumps({
                "Version": TAG,
                "Sha256": sha,
            }, indent=2))

        with gzip.open(f"./updates/{TAG}/{platform_name}-{arch}.gz", "wb") as f:
            f.write(exe_data)

        with open(f"./builds/{NAME}-{platform_name}-{arch}-{TAG}{ext}", "wb") as f:
            f.write(exe_data)
