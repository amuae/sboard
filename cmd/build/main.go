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

// 缓存的版本信息
var (
	singboxVersion string
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

// fetchLatestVersions 从 GitHub API 获取最新版本
func fetchLatestVersions(verbose bool) error {
	client := &http.Client{Timeout: 30 * time.Second}

	// 获取 sing-box 最新版本
	singboxURL := "https://api.github.com/repos/SagerNet/sing-box/releases/latest"
	if verbose {
		fmt.Printf("  请求: %s\n", singboxURL)
	}
	singboxVer, err := getLatestVersion(client, singboxURL)
	if err != nil {
		return fmt.Errorf("获取 sing-box 版本失败: %v", err)
	}
	// sing-box 的 tag 是 v1.12.12，需要去掉 v 前缀用于下载 URL
	singboxVersion = strings.TrimPrefix(singboxVer, "v")

	return nil
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
	fmt.Printf("\n下载 sing-box %s (%s/%s)...\n", singboxVersion, targetOS, targetArch)

	binaryName := "sing-box"
	if targetOS == "windows" {
		binaryName = "sing-box.exe"
	}

	// 转换架构名称为 sing-box 使用的格式
	// sing-box 支持: amd64, 386, arm64, armv5, armv6, armv7
	archName := targetArch
	switch targetArch {
	case "arm":
		archName = "armv7" // 默认使用 armv7
	case "armv5":
		archName = "armv5"
	case "armv6":
		archName = "armv6"
	case "armv7":
		archName = "armv7"
	case "386":
		archName = "386"
	case "mipsle":
		archName = "mipsle-softfloat"
	case "mips":
		archName = "mips-softfloat"
	}

	var filename string
	var extractFunc func(string, string, string) error

	switch targetOS {
	case "linux":
		filename = fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", singboxVersion, targetOS, archName)
		extractFunc = extractTarGz
	case "windows":
		filename = fmt.Sprintf("sing-box-%s-%s-%s.zip", singboxVersion, targetOS, archName)
		extractFunc = extractZipSingbox
	case "darwin":
		filename = fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", singboxVersion, targetOS, archName)
		extractFunc = extractTarGz
	case "freebsd":
		filename = fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", singboxVersion, targetOS, archName)
		extractFunc = extractTarGz
	default:
		fmt.Printf("  跳过 sing-box: 不支持的平台 %s/%s\n", targetOS, targetArch)
		return nil
	}

	baseURL := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/%s", singboxVersion, filename)
	targetPath := filepath.Join(embedDir, "sing-box", binaryName)

	// 下载文件
	tmpFile, err := downloadWithProxy(baseURL, verbose)
	if err != nil {
		fmt.Printf("  警告: sing-box 下载失败 (%v)，可能不支持此架构\n", err)
		return nil // 不阻塞构建
	}
	defer os.Remove(tmpFile)

	// 解压
	if err := extractFunc(tmpFile, targetPath, binaryName); err != nil {
		return err
	}

	// 设置可执行权限
	os.Chmod(targetPath, 0755)

	fmt.Printf("sing-box 下载完成: %s\n", targetPath)
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
