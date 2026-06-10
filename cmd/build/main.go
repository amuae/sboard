package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// GitHub 加速代理列表
var ghProxies = []string{
	"",                    // 直接下载
	"https://ghfast.top/", // 加速代理
}

// 嵌入目录
const embedDir = "cmd/agent/embed/configs"

// reF1nd sing-box fork 仓库路径（本地检出）
const ref1ndRepo = "https://github.com/reF1nd/sing-box.git"
const ref1ndBranch = "reF1nd-testing"

// reF1nd 标签匹配模式 — 优先使用带 -reF1nd 后缀的标签（reF1nd 专属发布）
const ref1ndTagSuffix = "-reF1nd"

// 构建 sing-box 使用的 build tags
// 注意：with_naive_outbound 在交叉编译 arm64 时会产生动态链接，导致 musl 系统不可用
const singboxBuildTagsUnix = "with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,with_cloudflared,badlinkname,tfogo_checklinkname0"
const singboxBuildTagsWindows = "with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,with_cloudflared,badlinkname,tfogo_checklinkname0"

func getSingboxBuildTags(targetOS string) string {
	if targetOS == "windows" {
		return singboxBuildTagsWindows
	}
	return singboxBuildTagsUnix
}

// 缓存的版本信息
var (
	singboxVersion string
	singboxCommit  string
)

// GitHubRelease GitHub Release API 响应结构
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func main() {
	var (
		targetOS     string
		targetArch   string
		skipFrontend bool
		skipDownload bool
		onlyDownload bool
		verbose      bool
	)

	flag.StringVar(&targetOS, "os", runtime.GOOS, "目标操作系统 (linux/windows/darwin)")
	flag.StringVar(&targetArch, "arch", runtime.GOARCH, "目标架构 (amd64/386/arm64/armv7/armv6)")
	flag.BoolVar(&skipFrontend, "skip-frontend", false, "跳过前端构建")
	flag.BoolVar(&skipDownload, "skip-download", false, "跳过核心下载")
	flag.BoolVar(&onlyDownload, "only-download", false, "只下载核心，不编译")
	flag.BoolVar(&verbose, "v", false, "详细输出")
	flag.Parse()

	fmt.Println("==========================================")
	fmt.Println("  Sboard 构建工具")
	fmt.Printf("  目标平台: %s/%s\n", targetOS, targetArch)
	fmt.Println("==========================================")

	// 1. 构建前端
	if !skipFrontend && !onlyDownload {
		if err := buildFrontend(); err != nil {
			fmt.Printf("前端构建失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 2. 下载核心
	if !skipDownload {
		if err := downloadCores(targetOS, targetArch, verbose); err != nil {
			fmt.Printf("核心下载失败: %v\n", err)
			os.Exit(1)
		}
	}

	if onlyDownload {
		fmt.Println("\n核心下载完成!")
		return
	}

	// 3. 编译 Sboard
	if err := buildSboard(); err != nil {
		fmt.Printf("Sboard 编译失败: %v\n", err)
		os.Exit(1)
	}

	// 4. 编译 Agent
	if err := buildAgent(targetOS, targetArch); err != nil {
		fmt.Printf("Agent 编译失败: %v\n", err)
		os.Exit(1)
	}

	showResult(targetOS, targetArch)
}

func buildFrontend() error {
	fmt.Println("\n=== 构建前端 ===")

	// 检查 web 目录是否存在
	if _, err := os.Stat("web"); os.IsNotExist(err) {
		fmt.Println("web 目录不存在，跳过前端构建")
		return nil
	}

	// npm run build
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = "web"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadCores(targetOS, targetArch string, verbose bool) error {
	fmt.Println("\n=== 下载核心文件 ===")

	// 获取最新版本
	fmt.Println("获取最新版本信息...")
	if err := fetchLatestVersions(verbose); err != nil {
		return fmt.Errorf("获取版本信息失败: %v", err)
	}
	fmt.Printf("  sing-box: %s\n", singboxVersion)

	// 清理旧文件
	cleanOldCores()

	// 下载 sing-box
	if err := downloadSingbox(targetOS, targetArch, verbose); err != nil {
		return fmt.Errorf("下载 sing-box 失败: %v", err)
	}

	return nil
}

// fetchLatestVersions 从 reF1nd 仓库获取最新标签版本
func fetchLatestVersions(verbose bool) error {
	// reF1nd 没有 release，直接从本地仓库获取最新 tag
	fmt.Println("  检测 reF1nd/sing-box 仓库...")

	repoDir, err := ensureReF1ndRepo(verbose)
	if err != nil {
		return fmt.Errorf("获取 reF1nd 仓库失败: %v", err)
	}

	// 获取最新标签
	tag, err := getLatestTag(repoDir, verbose)
	if err != nil {
		return fmt.Errorf("获取最新标签失败: %v", err)
	}

	singboxVersion = strings.TrimPrefix(tag, "v") // 去掉 v 前缀，保留版本号
	singboxCommit = tag
	if verbose {
		fmt.Printf("  reF1nd sing-box tag: %s\n", tag)
	}
	return nil
}

// getLatestTag 获取 reF1nd 仓库最新标签
// 优先使用带 -reF1nd 后缀的标签（reF1nd 专属发布），其次是普通 semver 标签
func getLatestTag(repoDir string, verbose bool) (string, error) {
	// 获取所有标签
	cmd := exec.Command("git", "tag", "--merged", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取标签列表失败: %v", err)
	}

	tags := strings.Split(strings.TrimSpace(string(out)), "\n")

	// 第一优先级：带 -reF1nd 后缀的标签
	var reF1ndTags []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if strings.HasSuffix(t, ref1ndTagSuffix) && t != "" {
			reF1ndTags = append(reF1ndTags, t)
		}
	}
	if len(reF1ndTags) > 0 {
		sort.Slice(reF1ndTags, func(i, j int) bool {
			return reF1ndTags[i] > reF1ndTags[j] // 降序
		})
		if verbose {
			fmt.Printf("  找到 reF1nd 标签: %s\n", reF1ndTags[0])
		}
		// 检出标签
		checkoutCmd := exec.Command("git", "checkout", "tags/"+reF1ndTags[0])
		checkoutCmd.Dir = repoDir
		if out, err := checkoutCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("检出标签 %s 失败: %s, %v", reF1ndTags[0], string(out), err)
		}
		return reF1ndTags[0], nil
	}

	// 第二优先级：普通 semver 标签（排除 alpha/beta/rc）
	var semverTags []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || strings.Contains(t, "alpha") || strings.Contains(t, "beta") || strings.Contains(t, "rc") {
			continue
		}
		semverTags = append(semverTags, t)
	}
	if len(semverTags) > 0 {
		sort.Slice(semverTags, func(i, j int) bool {
			return semverTags[i] > semverTags[j] // 降序
		})
		if verbose {
			fmt.Printf("  找到最新稳定标签: %s\n", semverTags[0])
		}
		checkoutCmd := exec.Command("git", "checkout", "tags/"+semverTags[0])
		checkoutCmd.Dir = repoDir
		if out, err := checkoutCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("检出标签 %s 失败: %s, %v", semverTags[0], string(out), err)
		}
		return semverTags[0], nil
	}

	// 第三优先级：HEAD
	if verbose {
		fmt.Println("  未找到标签，使用 HEAD")
	}
	headCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	headCmd.Dir = repoDir
	out, err = headCmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 HEAD 失败: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getLatestVersion 从 GitHub API 获取最新版本号
func getLatestVersion(client *http.Client, apiURL string) (string, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "sboard-build-tool")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

func cleanOldCores() {
	fmt.Println("清理旧的核心文件...")
	files := []string{
		filepath.Join(embedDir, "sing-box", "sing-box"),
		filepath.Join(embedDir, "sing-box", "sing-box.exe"),
	}
	for _, f := range files {
		os.Remove(f)
	}
}

func downloadSingbox(targetOS, targetArch string, verbose bool) error {
	fmt.Printf("\n编译 sing-box %s (%s/%s)...\n", singboxVersion, targetOS, targetArch)

	binaryName := "sing-box"
	if targetOS == "windows" {
		binaryName = "sing-box.exe"
	}

	targetPath := filepath.Join(embedDir, "sing-box", binaryName)

	// 转换架构名称
	goarch := targetArch
	goarm := ""
	switch targetArch {
	case "armv5":
		goarch = "arm"
		goarm = "5"
	case "armv6":
		goarch = "arm"
		goarm = "6"
	case "armv7":
		goarch = "arm"
		goarm = "7"
	}

	return buildSingboxFromReF1nd(targetOS, goarch, goarm, targetPath, verbose)
}

// buildSingboxFromSource 使用 CGO_ENABLED=0 从源码静态编译 sing-box，
// 生成的二进制兼容 musl libc (OpenWrt) 和 glibc 系统。
func buildSingboxFromSource(targetOS, goarch, goarm, version, targetPath string) error {
	armInfo := ""
	if goarm != "" {
		armInfo = " GOARM=" + goarm
	}
	fmt.Printf("\n从源码静态编译 sing-box v%s (%s/%s%s, CGO_ENABLED=0)...\n",
		version, targetOS, goarch, armInfo)

	// 使用临时 GOPATH，避免污染全局 module cache 并能精确定位输出二进制
	tmpGopath, err := os.MkdirTemp("", "sboard-gopath-*")
	if err != nil {
		return fmt.Errorf("创建临时 GOPATH 失败: %v", err)
	}
	defer os.RemoveAll(tmpGopath)

	pkg := fmt.Sprintf("github.com/sagernet/sing-box/cmd/sing-box@v%s", version)
	cmd := exec.Command("go", "install", pkg)
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+targetOS,
		"GOARCH="+goarch,
		"GOPATH="+tmpGopath,
	)
	if goarm != "" {
		cmd.Env = append(cmd.Env, "GOARM="+goarm)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("从源码编译 sing-box 失败: %v", err)
	}

	// 交叉编译时输出在 $GOPATH/bin/{GOOS}_{GOARCH}/，本机编译在 $GOPATH/bin/
	installedBin := filepath.Join(tmpGopath, "bin",
		fmt.Sprintf("%s_%s", targetOS, goarch), "sing-box")
	if _, err := os.Stat(installedBin); os.IsNotExist(err) {
		// 本机编译 fallback
		installedBin = filepath.Join(tmpGopath, "bin", "sing-box")
	}

	os.MkdirAll(filepath.Dir(targetPath), 0755)
	data, err := os.ReadFile(installedBin)
	if err != nil {
		return fmt.Errorf("读取编译结果失败 (%s): %v", installedBin, err)
	}
	if err := os.WriteFile(targetPath, data, 0755); err != nil {
		return fmt.Errorf("写入目标路径失败: %v", err)
	}

	fmt.Printf("sing-box 静态编译完成: %s\n", targetPath)
	return nil
}

// reF1ndSingboxDir 返回 reF1nd sing-box 仓库的本地路径
func reF1ndSingboxDir() string {
	// 优先使用 sboard 项目同级的目录，否则用当前工作目录
	candidates := []string{
		filepath.Join("..", "sing-box-ref1nd"),
		"/opt/github/sing-box-ref1nd",
	}
	for _, d := range candidates {
		abs, _ := filepath.Abs(d)
		if info, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil && !info.IsDir() {
			return abs
		}
	}
	return ""
}

// ensureReF1ndRepo 确保 reF1nd sing-box 仓库已克隆到本地
func ensureReF1ndRepo(verbose bool) (string, error) {
	repoDir := reF1ndSingboxDir()
	if repoDir != "" {
		// 已存在，拉取最新
		if verbose {
			fmt.Printf("  仓库已存在: %s\n", repoDir)
		}
		// 同时拉取分支和标签
		cmd := exec.Command("git", "fetch", "--tags", "origin", ref1ndBranch)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			fmt.Printf("  警告: git fetch 失败 (%v)，使用本地版本\n", err)
		} else {
			cmd = exec.Command("git", "checkout", ref1ndBranch)
			cmd.Dir = repoDir
			cmd.Run()
			cmd = exec.Command("git", "merge", "--ff-only", "origin/"+ref1ndBranch)
			cmd.Dir = repoDir
			if err := cmd.Run(); err != nil {
				fmt.Printf("  警告: git merge 失败 (%v)，使用本地版本\n", err)
			}
		}
		return repoDir, nil
	}

	// 需要克隆 — 用深度 50 确保能获取到最近 tag，然后单独拉取 tag 引用
	repoDir = filepath.Join("..", "sing-box-ref1nd")
	absDir, _ := filepath.Abs(repoDir)
	fmt.Printf("  克隆 reF1nd/sing-box (%s) 到 %s...\n", ref1ndBranch, absDir)
	cmd := exec.Command("git", "clone", "-b", ref1ndBranch, "--depth", "50", ref1ndRepo, absDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("克隆 reF1nd 仓库失败: %v", err)
	}

	// 拉取标签（浅克隆默认不包含 tag，单独获取标签引用）
	if verbose {
		fmt.Println("  获取标签信息...")
	}
	tagFetch := exec.Command("git", "fetch", "--tags", "origin", "+refs/tags/v*:refs/tags/v*")
	tagFetch.Dir = absDir
	if err := tagFetch.Run(); err != nil && verbose {
		fmt.Printf("  警告: tag fetch 失败 (%v)\n", err)
	}

	return absDir, nil
}

// buildSingboxFromReF1nd 从 reF1nd fork 编译 sing-box
func buildSingboxFromReF1nd(targetOS, goarch, goarm, targetPath string, verbose bool) error {
	repoDir, err := ensureReF1ndRepo(verbose)
	if err != nil {
		return err
	}

	armInfo := ""
	if goarm != "" {
		armInfo = " GOARM=" + goarm
	}
	fmt.Printf("\n从 reF1nd/sing-box 编译 (%s/%s%s, CGO_ENABLED=0)...\n",
		targetOS, goarch, armInfo)
	tags := getSingboxBuildTags(targetOS)
	fmt.Printf("  tags: %s\n", tags)

	// 在仓库目录中直接构建（用绝对路径避免 -o 被 cmd.Dir 影响）
	absTarget, _ := filepath.Abs(targetPath)
	execArgs := []string{"build", "-v", "-trimpath"}
	execArgs = append(execArgs, "-ldflags", fmt.Sprintf(
		"-checklinkname=0 -X 'github.com/sagernet/sing-box/constant.Version=ref1nd-%s' -s -w -buildid=", singboxVersion))
	execArgs = append(execArgs, "-tags", tags)
	execArgs = append(execArgs, "-o", absTarget, "./cmd/sing-box")

	cmd := exec.Command("go", execArgs...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+targetOS,
		"GOARCH="+goarch,
		"GOTOOLCHAIN=local",
	)
	if goarm != "" {
		cmd.Env = append(cmd.Env, "GOARM="+goarm)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("编译 sing-box (reF1nd fork) 失败: %v", err)
	}

	os.Chmod(targetPath, 0755)
	fmt.Printf("sing-box (reF1nd) 编译完成: %s\n", targetPath)
	return nil
}

func downloadWithProxy(baseURL string, verbose bool) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	var lastErr error
	for _, proxy := range ghProxies {
		url := proxy + baseURL
		if verbose {
			if proxy == "" {
				fmt.Printf("  尝试直接下载: %s\n", baseURL)
			} else {
				fmt.Printf("  尝试代理下载: %s\n", url)
			}
		}

		tmpFile, err := downloadFile(client, url)
		if err != nil {
			lastErr = err
			if verbose {
				fmt.Printf("  下载失败: %v\n", err)
			}
			continue
		}

		return tmpFile, nil
	}

	return "", fmt.Errorf("所有下载源都失败: %v", lastErr)
}

func downloadFile(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "sboard-download-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func extractTarGz(srcFile, destFile, binaryName string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 查找二进制文件
		if header.Typeflag == tar.TypeReg &&
			(strings.HasSuffix(header.Name, "/"+binaryName) || header.Name == binaryName) {
			os.MkdirAll(filepath.Dir(destFile), 0755)
			out, err := os.Create(destFile)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
	}

	return fmt.Errorf("未找到 %s", binaryName)
}

func extractZipSingbox(srcFile, destFile, binaryName string) error {
	r, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/"+binaryName) || filepath.Base(f.Name) == binaryName {
			return extractZipFile(f, destFile)
		}
	}

	return fmt.Errorf("未找到 %s", binaryName)
}

func extractZipFile(f *zip.File, destFile string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	os.MkdirAll(filepath.Dir(destFile), 0755)
	out, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func buildSboard() error {
	fmt.Println("\n=== 编译 Sboard ===")

	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", "sboard", "./cmd/sboard")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Println("编译完成: sboard")
	return nil
}

func buildAgent(targetOS, targetArch string) error {
	fmt.Printf("\n=== 编译 Agent (%s/%s) ===\n", targetOS, targetArch)

	outputName := "sboard-agent"
	if targetOS == "windows" {
		outputName = "sboard-agent.exe"
	}

	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", outputName, "./cmd/agent")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("编译完成: %s\n", outputName)
	return nil
}

func showResult(targetOS, targetArch string) {
	fmt.Println("\n==========================================")
	fmt.Println("  构建完成!")
	fmt.Println("==========================================")
	fmt.Printf("目标平台: %s/%s\n", targetOS, targetArch)
	fmt.Println()
	fmt.Println("嵌入的核心版本:")
	fmt.Printf("  - sing-box: v%s\n", singboxVersion)
	fmt.Println()
	fmt.Println("生成文件:")

	files := []string{"sboard", "sboard-agent"}
	if targetOS == "windows" {
		files = []string{"sboard.exe", "sboard-agent.exe"}
	}

	for _, f := range files {
		if info, err := os.Stat(f); err == nil {
			fmt.Printf("  %s (%d KB)\n", f, info.Size()/1024)
		}
	}
}
