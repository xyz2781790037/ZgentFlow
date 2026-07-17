package service

import "testing"

func TestNormalizeGeneratedSessionTitle(t *testing.T) {
	tests := map[string]string{
		"根据用户的问题，生成的短会话标题为：《Linux 网络编程》":                      "Linux 网络编程",
		"标题：\n**知识库检索方式**\n额外说明":                              "知识库检索方式",
		"<think>internal reasoning</think>\nTitle: API setup": "API setup",
		"`直接标题`": "直接标题",
	}

	for input, want := range tests {
		if got := normalizeGeneratedSessionTitle(input); got != want {
			t.Errorf("normalizeGeneratedSessionTitle(%q) = %q, want %q", input, got, want)
		}
	}
}
