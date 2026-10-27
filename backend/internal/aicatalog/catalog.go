// Package aicatalog defines the reusable applications, prompts and Markdown skills.
package aicatalog

import _ "embed"

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

type Application struct {
	Name        string
	Prompt      string
	SkillName   string
	SkillSource string
}

//go:embed skills/guidance.md
var guidance string

//go:embed skills/review.md
var review string

//go:embed skills/planning.md
var planning string

func Templates() []Template {
	return []Template{
		{ID: "guidance", Name: "解题引导", Description: "了解已有思路，逐步提示", Prompt: "你是本班的解题引导助手。先了解学生已经尝试的步骤，再用简短的问题和提示引导他完成下一步。不要直接给最终答案。依据本班材料时标注引用。"},
		{ID: "review", Name: "知识复习", Description: "梳理概念并检查理解", Prompt: "你是本班的知识复习助手。结合本班资料解释概念、比较易混淆的知识点，并给出一个简短自测问题。已有材料依据时标注引用；没有依据时说明限制。"},
		{ID: "planning", Name: "学习计划", Description: "按目标和时间安排学习", Prompt: "你是本班的学习计划助手。先确认学习目标、可用时间和当前基础，再给出可执行的阶段安排与检查方法。引用本班材料中的学习要求时标注出处，不编造课程安排。"},
	}
}

func Applications() []Application {
	templates := Templates()
	return []Application{
		{Name: "解题引导助手", Prompt: templates[0].Prompt, SkillName: "内置解题引导", SkillSource: guidance},
		{Name: "知识复习助手", Prompt: templates[1].Prompt, SkillName: "内置知识复习", SkillSource: review},
		{Name: "学习计划助手", Prompt: templates[2].Prompt, SkillName: "内置学习计划", SkillSource: planning},
	}
}
