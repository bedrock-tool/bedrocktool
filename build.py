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
print(flush=True)

LDFLAGS = f"-s -w -X github.com/bedrock-tool/bedrocktool/utils/updater.Version={TAG}"

PLATFORMS = [
    ("windows", ["amd64"], ".exe"),
    ("linux", ["amd64"], ""),
    #("darwin", ["amd64", "arm64"], ""),
    #("android", ["arm64"], ".apk"),
    #("js", ["wasip1"], "")
]


def clean():
    shutil.rmtree("./tmp", True)
    shutil.rmtree("./builds", True)
    shutil.rmtree("./updates", True)
    for file in os.listdir("./cmd/bedrocktool"):
        if file.endswith(".syso"):
            os.remove(f"./cmd/bedrocktool/{file}")

def make_dirs():
    os.mkdir("./tmp")
    os.mkdir("./builds")
    os.mkdir("./updates")


def build_cli(platform: str, arch: str, env_in: dict[str,str], tags: list[str], ldflags, compiled_path: str):
    env = {}
    env.update(env_in)
    env.update({
        "GOOS": platform,
        "GOARCH": arch,
    })
    args = [
        "go", "build",
        "-ldflags", ldflags,
        "-trimpath",
        "-tags", ",".join(tags),
        "-o", compiled_path,
        "-v"
    ]
    args.append("./cmd/bedrocktool")

    env2 = os.environ.copy()
    env2.update(env)
    subprocess.run(args, env=env2).check_returncode()


def build_gui(platform: str, arch: str, env, tags: list[str], ldflags, compiled_path: str):
    if platform == "windows":
        ldflags += " -H=windows"

    args = [
        "gogio",
        "-arch", arch,
        "-target", platform,
        "-icon", "icon.png",
        "-tags", ",".join(tags),
        "-ldflags", ldflags,
        "-o", compiled_path,
        "-x"
    ]
    if platform in ["android", "ios"]:
        args.extend(["-appid", APP_ID])
    args.append("./cmd/bedrocktool")

    env2 = os.environ.copy()
    env2.update(env)
    subprocess.run(args, env=env2).check_returncode()


def package(platform: str, arch: str, compiled_path: str, GUI: bool, ext: str):
    SUB1 = '-gui' if GUI else ''
    exe_out_path = f"./builds/{NAME}-{platform}-{arch}-{TAG}{SUB1}{ext}"

    if platform == "js":
        if GUI:
            shutil.copytree(compiled_path, exe_out_path)
        else:
            shutil.copy(compiled_path, exe_out_path)
        return

    # create hash and copy
    with open(compiled_path, "rb") as f:
        exe_data = f.read()
        sha = binascii.b2a_base64(hashlib.sha256(exe_data).digest()).decode("utf8").split("\n")[0]
    shutil.copy(compiled_path, exe_out_path)

    # create update
    updates_dir = f"./updates/{NAME}{SUB1}"
    os.makedirs(updates_dir, exist_ok=True)
    with open(f"{updates_dir}/{platform}-{arch}.json", "w") as f:
        f.write(json.dumps({
            "Version": TAG,
            "Sha256": sha,
        }, indent=2))
    
    # write update data
    os.makedirs(f"{updates_dir}/{TAG}", exist_ok=True)
    with gzip.open(f"{updates_dir}/{TAG}/{platform}-{arch}.gz", "wb") as f:
        f.write(exe_data)

    os.remove(compiled_path)


def build_all(platform_filter: str, arch_filter: str):
    for (platform, archs, ext) in PLATFORMS:
        if platform_filter and platform_filter != platform:
            continue
        archs = [a for a in archs if arch_filter == "" or a == arch_filter]
        if len(archs) == 0:
            continue
        for GUI in [False, True]:
            if platform in ["android", "js"] and not GUI:
                continue

            print(f"Building {platform} gui: {GUI}")
            SUB1 = '-gui' if GUI else ''
            name = f"{NAME}{SUB1}"

            tags = []
            if PACK_SUPPORT:
                tags.append("packs")
            if GUI:
                tags.append("gui")

            env = {
                "GOVCS": "*:off"
            }

            ldflags = LDFLAGS
            if tags.count("gui"):
                CmdName = "bedrocktool-gui"
            else:
                CmdName = "bedrocktool"
            ldflags += f" -X github.com/bedrock-tool/bedrocktool/utils/updater.CmdName={CmdName}"


            for arch in archs:
                compiled_path = f"./tmp/{platform}-{arch}{SUB1}/{name}{ext}"
                os.makedirs(os.path.dirname(compiled_path), exist_ok=True)

                if GUI and platform != "linux":
                    build_gui(platform, arch, env, tags, ldflags, compiled_path)
                else:
                    build_cli(platform, arch, env, tags, ldflags, compiled_path)
                
                package(platform, arch, compiled_path, GUI, ext)


def main():
    platform_filter = ""
    arch_filter = ""
    if len(sys.argv) > 1:
        platform_filter = sys.argv[1]

    if len(sys.argv) > 2:
        arch_filter = sys.argv[2]
    
    if platform_filter == "clean":
        clean()
        return

    clean()
    make_dirs()
    build_all(platform_filter, arch_filter)


main()