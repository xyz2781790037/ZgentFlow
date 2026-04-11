package tool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type ToolExecutor struct {
	WorkspaceDir string
}

func NewToolExecutor(workspace string) (*ToolExecutor, error) {
	if err := os.Mkdir(workspace, 0755); err != nil {
		return nil, fmt.Errorf("无法创建工作区: %v", err)
	}
	return &ToolExecutor{
		WorkspaceDir: workspace,
	}, nil
}

// ExecuteShell 在宿主机执行 Shell 命令，带有严格的超时熔断
func (e *ToolExecutor) ExecuteShell(ctx context.Context, command string, timeoutMs int64) ([]byte, error) {
	Log.Info("[Tool] 正在执行 Shell", command)
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Microsecond)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "zsh", "-c", command)
	cmd.Dir = e.WorkspaceDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if cmdCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("命令执行超时 (%d ms) 被强行熔断", timeoutMs)
	}
	if err != nil {
		return nil, fmt.Errorf("执行失败: %v\n错误日志: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ReadFile 读取工作区内的文件内容
func (e *ToolExecutor) ReadFile(filename string) ([]byte, error) {
	targetPath := fmt.Sprintf("%s/%s", e.WorkspaceDir, filename)
	return os.ReadFile(targetPath)
}

// WriteFile 将 Agent 生成的代码覆盖写入文件
func (e *ToolExecutor) WriteFile(filename string, content []byte) error {
	targetPath := fmt.Sprintf("%s/%s", e.WorkspaceDir, filename)
	return os.WriteFile(targetPath, content, 0644)
}
func (e *ToolExecutor) SearchWeb(ctx context.Context, query string) ([]byte, error) {
	Log.Info("发起联网搜索", "query", query)

	// 工业级防御：为外部网络调用设置绝对超时，防止 Goroutine 永久阻塞
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 针对天气查询的特化处理（因为通用搜索引擎抓取天气卡片往往被反爬屏蔽）
	// 这里做了一个极简的意图路由
	if strings.Contains(query, "天气") {
		// 提取城市名进行天气查询 (简化处理，实际应使用 NLP 提取)
		target := "Xian"
		if strings.Contains(query, "西安") {
			target = "Xian"
		}
		if strings.Contains(query, "北京") {
			target = "Beijing"
		}

		url := fmt.Sprintf("https://wttr.in/%s?format=3", target)
		req, _ := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("天气接口请求失败: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return body, nil
	}

	// 通用网页搜索底座 (使用无 API Key 的 DuckDuckGo HTML 版本)
	url := "https://html.duckduckgo.com/html/?q=" + query
	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	// 必须伪造 User-Agent，否则会被直接拦截
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("搜索引擎网络错误: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("搜索引擎返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 粗略清理 HTML 标签，提取文本（避免给大模型喂入过多无效 Token）
	// 注意：文件顶部需要引入 "regexp" 包
	re := regexp.MustCompile(`<[^>]*>`)
	cleanText := re.ReplaceAllString(string(body), " ")

	// 截取前 2000 个字符返回，防止大模型上下文超载
	if len(cleanText) > 2000 {
		cleanText = cleanText[:2000]
	}

	return []byte(cleanText), nil
}
