package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	AppName           = "bedrocktool"
	AppID             = "yuv.pink.bedrocktool"
	AndroidAPILevel   = "21"
	GitTagExcludeExpr = "r*"
)

var (
	// VER_RE matches vX.Y.Z(-N-gHASH)
	versionRegexp = regexp.MustCompile(`v(\d\.\d+\.\d+)(?:-(\d+)-(\w+))?`)
)

var gogioCmd string

func init() {
	var exeExt = ""
	if runtime.GOOS == "windows" {
		exeExt = ".exe"
	}
	gogioCmd = "./gogio" + exeExt
}

// Build represents a specific build configuration.
type Build struct {
	OS   string
	Arch string
	Type string
}

func (b Build) ExeExt() string {
	if b.OS == "windows" {
		return ".exe"
	}
	if b.OS == "android" {
		return ".apk"
	}
	if b.OS == "darwin" {
		return ".app"
	}
	return ""
}

func (b Build) LibExt() string {
	if b.OS == "windows" {
		return ".dll"
	}
	return ".so"
}

// BuildConfig holds configuration and state for the build process.
type BuildConfig struct {
	AppVersion         string
	BuildTag           string
	IsLatest           bool
	PackSupportEnabled bool
	GitHubOutputFile   *os.File
}

func (c *BuildConfig) writeGitHubOutput(name, value string) {
	if c.GitHubOutputFile != nil {
		fmt.Fprintf(c.GitHubOutputFile, "%s=%s\n", name, value)
		c.GitHubOutputFile.Sync()
	} else {
		log.Printf("::notice file=build.go::GITHUB_OUTPUT %s=%s", name, value)
	}
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash for %s: %w", path, err)
	}

	hashSum := hash.Sum(nil)
	return base64.StdEncoding.EncodeToString(hashSum), nil
}

// findAndroidToolchain finds the clang compiler path within the Android NDK.
func findAndroidToolchain(ndkHome, arch, apiLevel string) (string, error) {
	hostSystem := runtime.GOOS
	hostArch := runtime.GOARCH

	// Determine host tag for NDK paths
	hostTag := ""
	switch hostSystem {
	case "linux":
		hostTag = "linux-x86_64"
	case "darwin":
		if hostArch == "arm64" { // Apple Silicon
			hostTag = "darwin-aarch64"
		} else { // Intel Mac
			hostTag = "darwin-x86_64"
		}
	case "windows":
		// NDK on Windows is typically 64-bit
		hostTag = "windows-x86_64"
	default:
		return "", fmt.Errorf("unsupported host system for Android NDK: %s", hostSystem)
	}

	// Determine target compiler prefix based on Go ARCH
	compilerPrefixMap := map[string]string{
		"arm":   "armv7a-linux-androideabi",
		"arm64": "aarch64-linux-android",
		"386":   "i686-linux-android",
		"amd64": "x86_64-linux-android",
	}
	compilerPrefix, ok := compilerPrefixMap[arch]
	if !ok {
		return "", fmt.Errorf("unsupported Android architecture: %s", arch)
	}

	// Construct the path: $ANDROID_NDK_HOME/toolchains/llvm/prebuilt/<host-tag>/bin/<prefix><api>-clang
	clangPath := filepath.Join(ndkHome, "toolchains", "llvm", "prebuilt", hostTag, "bin", fmt.Sprintf("%s%s-clang", compilerPrefix, apiLevel))

	// On Windows, the executable might be clang.cmd
	if hostSystem == "windows" {
		clangPathCmd := clangPath + ".cmd"
		if _, err := os.Stat(clangPathCmd); err == nil {
			return clangPathCmd, nil
		}
	}

	// Check for the exact path
	if _, err := os.Stat(clangPath); err == nil {
		return clangPath, nil
	}

	return "", fmt.Errorf("android NDK clang compiler not found for arch %s at %s", arch, clangPath)
}

func generateChangelog(tagName string) (string, error) {
	// Check if git command is available
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git command not found: %w", err)
	}

	// First, try to get the commit hash for the tag
	cmdRevParse := exec.Command("git", "rev-parse", tagName)
	tagCommitHashBytes, errRevParse := cmdRevParse.Output()
	if errRevParse != nil {
		// Tag not found, generate changelog from recent commits
		log.Printf("Warning: Tag %s does not exist. Generating changelog from recent commits.", tagName)
		cmdLog := exec.Command("git", "log", "--pretty=format:%h %s", "-n", "20")
		logOutputBytes, errLog := cmdLog.Output()
		if errLog != nil {
			return "", fmt.Errorf("error fetching recent commits: %w", errLog)
		}
		return "No tag found, showing recent commits:\n" + strings.TrimSpace(string(logOutputBytes)), nil
	}

	tagCommitHash := strings.TrimSpace(string(tagCommitHashBytes))

	// Get commits between the tag and HEAD
	cmdLogRange := exec.Command("git", "log", "--pretty=format:%h %s", fmt.Sprintf("%s..HEAD", tagCommitHash))
	commitsOutputBytes, errLogRange := cmdLogRange.Output()
	if errLogRange != nil {
		return "", fmt.Errorf("error fetching commits since tag %s: %w", tagName, errLogRange)
	}

	commitsOutput := strings.TrimSpace(string(commitsOutputBytes))
	if commitsOutput == "" {
		return fmt.Sprintf("No new commits found since tag %s", tagName), nil
	}

	// Format the changelog
	lines := strings.Split(commitsOutput, "\n")
	var changelogLines []string
	for _, line := range lines {
		if line != "" {
			changelogLines = append(changelogLines, "- "+line)
		}
	}

	return strings.Join(changelogLines, "\n"), nil
}

func (c *BuildConfig) getVersion() error {
	// Check if git command is available
	if _, err := exec.LookPath("git"); err != nil {
		log.Printf("Warning: git command not found. Using default version: 0.0.0")
		c.AppVersion = "0.0.0"
		c.BuildTag = "0.0.0-0"
		c.IsLatest = false
		c.writeGitHubOutput("release_tag", "r0.0.0")
		c.writeGitHubOutput("is_latest", "false")
		return nil // Not a fatal error for the build script itself
	}

	// git describe --tags --always --exclude 'r*'
	cmdDescribe := exec.Command("git", "describe", "--tags", "--always", "--exclude", GitTagExcludeExpr)
	gitDescribeBytes, err := cmdDescribe.Output()
	if err != nil {
		log.Printf("Error running git describe: %v. Using default version.", err)
		c.AppVersion = "0.0.0"
		c.BuildTag = "0.0.0-0"
		c.IsLatest = false
		c.writeGitHubOutput("release_tag", "r0.0.0")
		c.writeGitHubOutput("is_latest", "false")
		return nil // Not a fatal error
	}
	gitTag := strings.TrimSpace(string(gitDescribeBytes))

	verMatch := versionRegexp.FindStringSubmatch(gitTag)
	if verMatch == nil {
		log.Printf("Warning: git describe output '%s' does not match version regex. Using default version.", gitTag)
		c.AppVersion = "0.0.0"
		patch := "0"
		// If it's just a commit hash, use it in the tag
		if regexp.MustCompile(`^[0-9a-f]{7,40}$`).MatchString(gitTag) {
			c.BuildTag = fmt.Sprintf("0.0.0-%s-%s", patch, gitTag[:min(7, len(gitTag))])
		} else {
			c.BuildTag = fmt.Sprintf("0.0.0-%s", patch)
		}
	} else {
		c.AppVersion = verMatch[1]
		patch := verMatch[2]
		if patch == "" {
			patch = "0"
		}
		commitHash := verMatch[3]
		c.BuildTag = fmt.Sprintf("%s-%s", c.AppVersion, patch)
		if commitHash != "" {
			c.BuildTag += "-" + commitHash
		}
	}

	// Determine if the current commit is on the 'master' or 'main' branch
	cmdBranch := exec.Command("git", "branch", "--show-current")
	branchBytes, err := cmdBranch.Output()
	if err != nil {
		log.Printf("Warning: Could not determine active branch: %v", err)
		c.IsLatest = false
	} else {
		activeBranch := strings.TrimSpace(string(branchBytes))
		c.IsLatest = (activeBranch == "master" || activeBranch == "main")
	}

	// Set GitHub Actions outputs
	c.writeGitHubOutput("release_tag", "r"+c.AppVersion) // Release tag format rX.Y.Z
	c.writeGitHubOutput("build_tag", c.BuildTag)         // Build tag format V.Y.Z-PATCH(-gHASH)
	c.writeGitHubOutput("is_latest", fmt.Sprintf("%t", c.IsLatest))

	return nil
}

func checkPackSupport() (bool, error) {
	filePath := filepath.Join("subcommands", "resourcepack-d", "resourcepack-d.go")
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Warning: %s not found. Pack support disabled.", filePath)
			return false, nil
		}
		return false, fmt.Errorf("error reading %s: %w", filePath, err)
	}
	return bytes.Contains(content[:min(len(content), 1024)], []byte("package ")), nil
}

func cleanSyso() {
	sysoDir := filepath.Join("cmd", AppName)
	files, err := os.ReadDir(sysoDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading directory %s: %v", sysoDir, err)
		}
		return
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".syso") {
			filePath := filepath.Join(sysoDir, file.Name())
			log.Printf("Removing %s", filePath)
			if err := os.Remove(filePath); err != nil {
				log.Printf("Error removing %s: %v", filePath, err)
			}
		}
	}
}

func libBuildCmd(buildTag string, build Build, ldflags, tags []string) (string, []string) {
	outputFilename := fmt.Sprintf("libbedrocktool-%s-%s-%s", build.OS, build.Arch, buildTag)
	outputFilename += build.LibExt()
	outputPath := filepath.Join("builds", outputFilename)

	return outputPath, []string{
		"go", "build",
		"-buildmode", "c-shared",
		"-ldflags", strings.Join(ldflags, " "),
		"-tags", strings.Join(tags, ","),
		"-o", outputPath,
		"-v",
		"-trimpath",
		"./cmd/libbedrocktool",
	}
}

func cliBuildCmd(buildTag string, build Build, ldflags, tags []string) (string, []string) {
	outputFilename := fmt.Sprintf("bedrocktool-%s-%s-%s", build.OS, build.Arch, buildTag)
	outputFilename += build.ExeExt()
	outputPath := filepath.Join("builds", outputFilename)

	return outputPath, []string{
		"go", "build",
		"-ldflags", strings.Join(ldflags, " "),
		"-tags", strings.Join(tags, ","),
		"-o", outputPath,
		"-v",
		"-trimpath",
		"./cmd/bedrocktool",
	}
}

func guiBuildCmd(buildTag string, build Build, ldflags, tags []string) (string, []string) {
	outputFilename := fmt.Sprintf("bedrocktool-gui-%s-%s-%s", build.OS, build.Arch, buildTag)
	outputFilename += build.ExeExt()
	outputPath := filepath.Join("builds", outputFilename)

	if build.OS == "linux" {
		return outputPath, []string{
			"go", "build",
			"-ldflags", strings.Join(ldflags, " "),
			"-tags", strings.Join(tags, ","),
			"-o", outputPath,
			"-v",
			"-trimpath",
			"./cmd/bedrocktool",
		}
	}

	buildCmd := []string{
		gogioCmd,
	}
	if build.OS == "android" {
		buildCmd = append(buildCmd, "-target", build.OS)
		buildCmd = append(buildCmd, "-appid", AppID)
		buildCmd = append(buildCmd, "-resources", "./android-resources")
	}
	tagSplit := strings.Split(strings.ReplaceAll(buildTag, "-", "."), ".")
	tagSplit = tagSplit[:len(tagSplit)-1]
	if len(tagSplit) < 4 {
		tagSplit = append(tagSplit, "0")
	}
	if build.OS == "android" {
		tagSplit[3] = strings.Join(tagSplit, "")
	}
	gioVersion := strings.Join(tagSplit, ".")
	gioTarget := build.OS
	if build.OS == "darwin" {
		gioTarget = "macos"
	}

	buildCmd = append(buildCmd,
		"-arch", build.Arch,
		"-target", gioTarget,
		"-version", gioVersion,
		"-icon", "icon.png",
		"-tags", strings.Join(tags, ","),
		"-ldflags", strings.Join(ldflags, " "),
		"-o", outputPath,
		"-x",
		"./cmd/bedrocktool",
	)

	return outputPath, buildCmd
}

func runCommand(buildCmd, env []string, cwd string) error {
	log.Printf("Executing: %s", strings.Join(buildCmd, " "))
	cmd := exec.Command(buildCmd[0], buildCmd[1:]...)
	cmd.Env = env
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build command failed: %w", err)
	}
	return nil
}

func androidEnv(build Build, env *[]string) error {
	ndkHome := os.Getenv("ANDROID_NDK_HOME")
	if ndkHome == "" {
		log.Println("Error: ANDROID_NDK_HOME environment variable not set. Skipping Android build.")
		return nil
	}
	clangCompilerPath, err := findAndroidToolchain(ndkHome, build.Arch, AndroidAPILevel)
	if err != nil {
		log.Printf("Error finding Android NDK toolchain: %v. Skipping Android build.", err)
		return nil
	}
	log.Printf("Using Android NDK compiler: %s", clangCompilerPath)
	*env = append(*env, "CC="+clangCompilerPath)
	*env = append(*env, "CXX="+clangCompilerPath)
	*env = append(*env, "CGO_ENABLED=1")
	return nil
}

func (c *BuildConfig) doBuild(build Build) error {
	buildName := fmt.Sprintf("%s-%s-%s", build.OS, build.Arch, build.Type)
	log.Printf("\n--- Building %s ---", buildName)

	env := os.Environ()
	env = append(env, "GOVCS=*:off")
	env = append(env, "GOOS="+build.OS)
	env = append(env, "GOARCH="+build.Arch)

	var tags []string
	if c.PackSupportEnabled {
		tags = append(tags, "packs")
	}
	if build.Type == "gui" {
		tags = append(tags, "gui")
	}

	ldflags := []string{"-s", "-w"}
	ldflags = append(ldflags, fmt.Sprintf("-X github.com/bedrock-tool/%s/utils.Version=%s", AppName, c.BuildTag))

	cmdName := AppName
	if build.Type != "cli" {
		cmdName += "-" + build.Type
	}
	ldflags = append(ldflags, fmt.Sprintf("-X github.com/bedrock-tool/%s/utils.CmdName=%s", AppName, cmdName))

	var outputPath string
	var buildCmd []string
	switch build.OS {
	case "android":
		err := androidEnv(build, &env)
		if err != nil {
			return err
		}
		switch build.Type {
		case "lib":
			outputPath, buildCmd = libBuildCmd(c.BuildTag, build, ldflags, tags)
		case "gui":
			outputPath, buildCmd = guiBuildCmd(c.BuildTag, build, ldflags, tags)
		default:
			return fmt.Errorf("%s not supported on android", build.Type)
		}

	case "windows", "linux", "darwin":
		switch build.Type {
		case "lib":
			outputPath, buildCmd = libBuildCmd(c.BuildTag, build, ldflags, tags)
		case "gui":
			if build.OS == "windows" {
				cleanSyso()
				ldflags = append(ldflags, "-H=windows")
			}
			outputPath, buildCmd = guiBuildCmd(c.BuildTag, build, ldflags, tags)
		case "cli":
			outputPath, buildCmd = cliBuildCmd(c.BuildTag, build, ldflags, tags)
		default:
			return fmt.Errorf("unknown build type %s", build.Type)
		}

	default:
		return fmt.Errorf("unknown os %s", build.OS)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	err := runCommand(buildCmd, env, "")
	if err != nil {
		return err
	}

	log.Printf("Successfully built: %s", outputPath)

	if build.OS != "wasm" && build.OS != "android" {
		err := createUpdate(build, outputPath, c.BuildTag)
		if err != nil {
			return err
		}
	}

	if build.Type == "gui" && build.OS == "windows" {
		cleanSyso()
	}

	return nil
}

func createUpdate(build Build, outputPath, buildTag string) error {
	exeHash, err := sha256File(outputPath)
	if err != nil {
		log.Printf("Warning: Could not calculate file hash for %s: %v. Skipping update file creation.", outputPath, err)
		return nil
	}

	updatesDirName := AppName
	if build.Type == "gui" {
		updatesDirName += "-gui"
	}
	updatesBaseDir := "updates"
	updatesDir := filepath.Join(updatesBaseDir, updatesDirName)
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create updates directory %s: %w", updatesDir, err)
	}

	updateInfoPath := filepath.Join(updatesDir, fmt.Sprintf("%s-%s.json", build.OS, build.Arch))
	updateInfo := map[string]string{
		"Version": buildTag,
		"Sha256":  exeHash,
	}
	updateInfoBytes, err := json.MarshalIndent(updateInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal update info JSON for %s: %w", updateInfoPath, err)
	}
	if err := os.WriteFile(updateInfoPath, updateInfoBytes, 0644); err != nil {
		return fmt.Errorf("failed to write update info file %s: %w", updateInfoPath, err)
	}
	log.Printf("Generated update info file: %s", updateInfoPath)

	compressedUpdatesDir := filepath.Join(updatesDir, buildTag)
	if err := os.MkdirAll(compressedUpdatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create compressed updates directory %s: %w", compressedUpdatesDir, err)
	}
	compressedFilePath := filepath.Join(compressedUpdatesDir, fmt.Sprintf("%s-%s.gz", build.OS, build.Arch))

	compressedFile, err := os.Create(compressedFilePath)
	if err != nil {
		return fmt.Errorf("failed to create compressed update file %s: %w", compressedFilePath, err)
	}
	defer compressedFile.Close()

	gzipWriter := gzip.NewWriter(compressedFile)
	defer gzipWriter.Close()

	exeFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("failed to open executable for compression %s: %w", outputPath, err)
	}
	defer exeFile.Close()

	if _, err := io.Copy(gzipWriter, exeFile); err != nil {
		return fmt.Errorf("failed to compress executable to %s: %w", compressedFilePath, err)
	}
	log.Printf("Generated compressed update file: %s", compressedFilePath)
	return nil
}

func buildGogio() error {
	var env []string
	env = append(env, os.Environ()...)
	err := runCommand([]string{
		"go", "build",
		"-o", gogioCmd,
		"-v",
		"-trimpath",
		"./gio-cmd/gogio",
	}, env, "")
	if err != nil {
		return err
	}
	return nil
}

func main() {
	log.SetFlags(0)
	log.Println("--- Starting Build Process ---")

	if false {
		if err := buildGogio(); err != nil {
			log.Fatalf("building gogio %s", err)
		}
	}
	gogioCmd = "gogio"

	buildCfg := &BuildConfig{}

	githubOutputPath := os.Getenv("GITHUB_OUTPUT")
	if githubOutputPath != "" {
		var err error
		buildCfg.GitHubOutputFile, err = os.OpenFile(githubOutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Warning: Could not open GITHUB_OUTPUT file %s: %v", githubOutputPath, err)
		} else {
			defer buildCfg.GitHubOutputFile.Close()
		}
	} else {
		log.Println("GITHUB_OUTPUT environment variable not set. Writing outputs to stderr.")
	}

	log.Println("Cleaning existing builds and updates directories...")
	if err := os.RemoveAll("builds"); err != nil && !os.IsNotExist(err) {
		log.Printf("Error cleaning builds directory: %v", err)
	}
	if err := os.MkdirAll("builds", 0755); err != nil {
		log.Fatalf("Error creating builds directory: %v", err)
	}
	if err := os.RemoveAll("updates"); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error cleaning updates directory: %v", err)
	}
	if err := os.MkdirAll("updates", 0755); err != nil {
		log.Fatalf("Error creating updates directory: %v", err)
	}

	if err := buildCfg.getVersion(); err != nil {
		log.Fatalf("Failed to get version information: %v", err)
	}
	log.Printf("App Version: %s", buildCfg.AppVersion)
	log.Printf("Build Tag: %s", buildCfg.BuildTag)

	var err error
	buildCfg.PackSupportEnabled, err = checkPackSupport()
	if err != nil {
		log.Fatalf("Failed to check pack support: %v", err)
	}
	log.Printf("Pack Support Enabled: %t", buildCfg.PackSupportEnabled)

	allBuilds := []Build{
		// Desktop GUI builds
		{"windows", "amd64", "gui"},
		{"linux", "amd64", "gui"},
		//{"android", "arm64", "gui"},
		{"darwin", "amd64", "gui"},
		{"darwin", "arm64", "gui"},

		// Desktop CLI builds
		{"windows", "amd64", "cli"},
		{"linux", "amd64", "cli"},
		{"linux", "arm64", "cli"},
		{"linux", "arm", "cli"},
		{"darwin", "amd64", "cli"},
		{"darwin", "arm64", "cli"},

		// Desktop LIB builds
		//{"windows", "amd64", "lib"},
		//{"linux", "amd64", "lib"},

		// Android Library builds
		//{"android", "arm64", "lib"},
		//{"android", "arm", "lib"},

		// {"wasm", "wasm", true},
	}

	buildsToRun := allBuilds
	var selectedOSes []string
	var selectedTypes []string
	if len(os.Args) > 1 {
		selectedOSes = strings.Split(strings.ToLower(os.Args[1]), ",")
		if len(os.Args) > 2 {
			selectedTypes = strings.Split(strings.ToLower(os.Args[2]), ",")
		}
	}

	shouldRunBuild := func(build Build) bool {
		if len(selectedOSes) > 0 && !slices.Contains(selectedOSes, build.OS) {
			return false
		}
		if len(selectedTypes) > 0 && !slices.Contains(selectedTypes, build.Type) {
			return false
		}
		return true
	}

	if len(selectedOSes)+len(selectedTypes) == 0 {
		log.Println("Building all configurations...")
	}

	for _, build := range buildsToRun {
		if !shouldRunBuild(build) {
			continue
		}
		if build.OS == "linux" && build.Type != "cli" && runtime.GOOS == "windows" {
			log.Printf("Skipping Linux GUI build on Windows host.")
			continue
		}
		if build.OS == "android" && os.Getenv("ANDROID_NDK_HOME") == "" {
			log.Printf("Skipping Android build for %s because ANDROID_NDK_HOME is not set.", build.Arch)
			continue
		}
		if err := buildCfg.doBuild(build); err != nil {
			log.Fatalf("Build failed for %s-%s-%s: %v", build.OS, build.Arch, build.Type, err)
		}
	}

	changelogTagName := "v" + strings.Split(buildCfg.AppVersion, "-")[0]
	changelog, err := generateChangelog(changelogTagName)
	if err != nil {
		log.Printf("Warning: Failed to generate changelog: %v", err)
		changelog = fmt.Sprintf("Failed to generate changelog: %v", err)
	}

	log.Println("\n--- Changelog ---")
	fmt.Println(changelog)

	changelogFilePath := "changelog.txt"
	header := fmt.Sprintf("## Commits since %s\n", changelogTagName)
	if err := os.WriteFile(changelogFilePath, []byte(header+changelog), 0644); err != nil {
		log.Printf("Error writing changelog to file %s: %v", changelogFilePath, err)
	} else {
		log.Printf("Changelog saved to %s", changelogFilePath)
	}

	log.Println("\n--- Build Process Finished ---")
}
