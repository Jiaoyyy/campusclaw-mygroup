package tutor

import (
	"strings"
	"testing"

	"campusclaw/backend/internal/retrieval"
)

func TestSkillParsingAndRouting(t *testing.T) {
	source := "名称：数学解题引导\n适用条件：方程,去括号\n启用：是\n\n先指出下一步，不给最终答案。"
	math, err := parseSkill(source)
	if err != nil || math.Name != "数学解题引导" || !math.Enabled {
		t.Fatalf("parse skill: %+v %v", math, err)
	}
	math.ID = 2
	other := math
	other.ID = 1
	other.Name = "另一种引导"
	other.TriggerText = "去括号"
	selected := route("这道方程怎么去括号", []Skill{math, other})
	if selected == nil || selected.ID != 2 {
		t.Fatalf("expected higher lexical score, got %+v", selected)
	}
	if _, err := parseSkill(strings.Replace(source, "启用：是", "启用：是\n启用：否", 1)); err == nil {
		t.Fatal("duplicate field accepted")
	}
	if _, err := parseSkill(strings.Replace(source, "启用：是", "工具：shell\n启用：是", 1)); err == nil {
		t.Fatal("unsafe tool accepted")
	}
}

func TestAssembleSeparatesEvidenceAndPrompt(t *testing.T) {
	a := Agent{Prompt: "你是老师"}
	skill := &Skill{Name: "数学解题引导", Description: "只提示下一步"}
	hits := []retrieval.Hit{{Title: "数学材料", ChunkIndex: 1, ChunkText: "去括号要分配乘法", CitationNumber: 1}}
	messages := assemble(a, skill, "下一步？", []ChatMessage{{Role: "user", Content: "下一步？"}}, hits)
	if len(messages) != 2 || strings.Contains(messages[0].Content, "去括号要分配乘法") || !strings.Contains(messages[1].Content, "[1] 材料") {
		t.Fatalf("evidence was not kept in user message: %+v", messages)
	}
	empty := assemble(a, nil, "没有资料？", nil, nil)
	if !strings.Contains(empty[0].Content, "不得编造出处") || strings.Contains(empty[1].Content, "[1]") {
		t.Fatal("no-evidence branch is wrong")
	}
}
