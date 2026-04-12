package tool

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
)

// SearchSkill 实现了 Skill 接口
type SearchSkill struct {}

func (s *SearchSkill) Name() string { return "search_web" }

// 这里极其重要：大模型就是通过看这段话，来知道自己“不仅能查天气，还能查一切”的！
func (s *SearchSkill) Description() string { 
	return "互联网搜索引擎。当你需要获取客观事实、实时资讯、天气、技术文档或任何你不知道的信息时，必须使用此技能。" 
}

func (s *SearchSkill) Parameters() string {
	return `{"type": "object", "properties": {"query": {"type": "string", "description": "搜索关键词"}}, "required": ["query"]}`
}

func (s *SearchSkill) Execute(ctx context.Context, args string) (string, error) {
	var params struct{ Query string }
	_ = json.Unmarshal([]byte(args), &params)

	// 这里复用你之前写的 duckduckgo 搜索代码
	url := "https://html.duckduckgo.com/html/?q=" + params.Query
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`<[^>]*>`)
	cleanText := re.ReplaceAllString(string(body), " ")
	if len(cleanText) > 2000 { cleanText = cleanText[:2000] }
	return cleanText, nil
}