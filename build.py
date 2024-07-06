import subprocess, re, os, shutil, json, binascii, hashlib, gzip, sys
from collections import namedtuple
import git

NAME = "bedrocktool"
APP_ID = "yuv.pink.bedrocktool"

VER_RE = re.compile(r"v(\d\.\d+\.\d+)(?:-(\d+)-(\w))?")

Build = namedtuple("Build", ["os", "arch", "gui"])

GITHUB_OUTPUT = os.getenv("GITHUB_OUTPUT")
if GITHUB_OUTPUT:
    GITHUB_OUTPUT = open(GITHUB_OUTPUT, "a")

def generate_changelog(tag_name):
    repo = git.Repo(".")
    
    try:
        tag = repo.tags[tag_name]
    except IndexError:
        raise ValueError(f"Tag {tag_name} does not exist in the repository")

    commits_since_tag = list(repo.iter_commits(rev=f'{tag_name}..HEAD'))

    if not commits_since_tag:
        return f"No commits found since tag {tag_name}"

    changelog = []
    for commit in commits_since_tag:
        changelog.append(f"- {commit.hexsha[:7]} {commit.message.strip()}")

    return '\n'.join(changelog)


def get_version():
    repo = git.Repo(".")
    GIT_TAG = subprocess.run(["git", "describe", "--exclude", "r*", "--tags", "--always"], stdout=subprocess.PIPE).stdout.decode("utf8").split("\n")[0]
    if GIT_TAG == "":
        GIT_TAG = "v0.0.0"
    VER_MATCH = VER_RE.match(GIT_TAG)
    VER = VER_MATCH.group(1)
    PATCH = VER_MATCH.group(2) or "0"
    TAG = f"{VER}-{PATCH}"
    if GITHUB_OUTPUT:
        GITHUB_OUTPUT.write(f"release_tag=r{VER}\n")
        
        GITHUB_OUTPUT.write(f"is_latest={'true' if repo.active_branch == 'master' else 'false'}\n")
    return VER, TAG


def check_pack_support():
    with open("./subcommands/resourcepack-d/resourcepack-d.go", "rb") as f:
        return f.read(100).count(b"package ") > 0


def clean_syso():
    for file in os.listdir("./cmd/bedrocktool"):
        if file.endswith(".syso"):
            os.remove(f"./cmd/bedrocktool/{file}")

def build_wasm(build: Build, tags: list[str], env: dict, ldflags: list[str]):
    env.update({
        "GOOS": build.os,
        "GOARCH": build.arch
    })
    args = [
        "go", "build",
        "-ldflags", " ".join(ldflags),
        "-trimpath",
        "-tags", ",".join(tags),
        "-o", "tmp/bedrocktool.wasm",
        "-v",
        "./cmd/bedrocktool"
    ]
    subprocess.run(args, env=env).check_returncode()


def do_build(build: Build):
    print("Building", build)
    env = dict()
    env.update(os.environ.copy())
    env["GOVCS"] = "*:off"

    tags = list()
    if PACK_SUPPORT:
        tags.append("packs")

    ldflags = list()
    ldflags.append(f"-s -w -X github.com/bedrock-tool/bedrocktool/utils/updater.Version={TAG}")

    if build.gui:
        CmdName = "bedrocktool-gui"
        tags.append("gui")
    else:
        CmdName = "bedrocktool"
    ldflags.append(f"-X github.com/bedrock-tool/bedrocktool/utils/updater.CmdName={CmdName}")

    if build.arch == "wasm":
        return build_wasm(build, tags, env, ldflags)

    ext = {
        "windows": ".exe",
        "linux": ""
    }[build.os]

    compiled_path = f"./builds/{NAME}-{build.os}-{build.arch}-{TAG}{'-gui' if build.gui else ''}{ext}"

    if build.gui and build.os != "linux":
        clean_syso()
        if build.os == "windows":
            ldflags.append("-H=windows")

        args = [
            "gogio",
            "-arch", build.arch,
            "-target", build.os,
            "-icon", "icon.png",
            "-tags", ",".join(tags),
            "-ldflags", " ".join(ldflags),
            "-o", compiled_path,
            "-x"
        ]
        if build.os in ["android", "ios"]:
            args.extend(["-appid", APP_ID])
    else:
        env.update({
            "GOOS": build.os,
            "GOARCH": build.arch
        })
        args = [
            "go", "build",
            "-ldflags", " ".join(ldflags),
            "-trimpath",
            "-tags", ",".join(tags),
            "-o", compiled_path,
            "-v"
        ]

    args.append("./cmd/bedrocktool")
    subprocess.run(args, env=env).check_returncode()
    if build.gui and build.os == "windows":
        clean_syso()

    exe_hash = sha256_file(compiled_path)

    # create update
    updates_dir = f"./updates/{NAME}{'-gui' if build.gui else ''}"
    os.makedirs(updates_dir, exist_ok=True)
    with open(f"{updates_dir}/{build.os}-{build.arch}.json", "w") as f:
        f.write(json.dumps({
            "Version": TAG,
            "Sha256": exe_hash,
        }, indent=2))
    
    # write update data
    os.makedirs(f"{updates_dir}/{TAG}", exist_ok=True)
    with gzip.open(f"{updates_dir}/{TAG}/{build.os}-{build.arch}.gz", "wb") as f:
        with open(compiled_path, "rb") as fr:
            f.write(fr.read())


def main():
    os.makedirs("./builds", exist_ok=True)
    os.makedirs("./updates", exist_ok=True)

    builds = [
        Build("windows", "amd64", True),
        Build("linux", "amd64", True),
        Build("windows", "amd64", False),
        Build("linux", "amd64", False),
        #Build("js", "wasm", True)
    ]

    selected_os = sys.argv[1] if len(sys.argv) > 1 else ""
    if selected_os:
        builds = [build for build in builds if build.os == selected_os]
    if len(builds) == 0:
        print("no build selected")
        return

    shutil.rmtree("./updates", True)
    for build in builds:
        do_build(build)
    changelog = generate_changelog("v"+VER)
    print(changelog)
    with open("./changelog.txt", "w") as f:
        f.write("## Commits\n")
        f.write(changelog)


def sha256_file(path: str) -> str:
    with open(path, "rb") as f:
        exe_data = f.read()
        return binascii.b2a_base64(hashlib.sha256(exe_data).digest()).decode("utf8").split("\n")[0]


VER, TAG = get_version()
print(f"VER: {VER}")
print(f"TAG: {TAG}")

PACK_SUPPORT = check_pack_support()
print(f"Pack Support: {PACK_SUPPORT}")

main()
