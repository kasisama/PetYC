// Package playermsg owns player-visible reusable messages. Keeping templates
// here lets chat handlers and the admin preview use the same message keys and
// sample variables without exposing service errors.
package playermsg

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"
)

type Definition struct {
	Key             string            `json:"key"`
	Description     string            `json:"description"`
	Kind            string            `json:"kind"`
	Tone            string            `json:"tone"`
	Template        string            `json:"template"`
	Variants        []string          `json:"variants,omitempty"`
	Variables       map[string]string `json:"variables"`
	ImageCandidates []string          `json:"image_candidates,omitempty"`
	NextActions     []string          `json:"next_actions,omitempty"`
}

var definitions = map[string]Definition{
	"system.temporarily_unavailable": {
		Key: "system.temporarily_unavailable", Description: "命令发生技术故障时的安全回复",
		Kind: "technical_error", Tone: "克制",
		Template:  "这次操作没有完成，请稍后再试。\n如果仍然遇到问题，可以发送“{{.Command}}”重试。",
		Variables: map[string]string{"Command": "宠物菜单"},
	},
	"error.pet_required": {
		Key: "error.pet_required", Description: "玩家尚未领养宠物",
		Kind: "business_rejected", Tone: "友好", NextActions: []string{"领养宠物"},
		Template: "你还没有同行伙伴。\n发送“领养宠物”选择第一位伙伴。",
	},
	"error.expedition_active": {
		Key: "error.expedition_active", Description: "已有行动进行中",
		Kind: "business_rejected", Tone: "友好", NextActions: []string{"我的宠物"},
		Template: "宠物正在进行其他行动。\n发送“我的宠物”查看当前进度。",
	},
	"error.expedition_not_ready": {
		Key: "error.expedition_not_ready", Description: "远征尚未结束",
		Kind: "business_rejected", Tone: "友好", NextActions: []string{"远征状态"},
		Template: "远征还在进行中。\n发送“远征状态”查看预计返回时间。",
	},
	"error.nothing_to_claim": {
		Key: "error.nothing_to_claim", Description: "没有可领取的远征奖励",
		Kind: "business_rejected", Tone: "友好", NextActions: []string{"远征"},
		Template: "当前没有可领取的远征奖励。\n发送“远征”安排下一次行动。",
	},
	"error.insufficient_item": {
		Key: "error.insufficient_item", Description: "背包物品不足",
		Kind: "business_rejected", Tone: "友好", NextActions: []string{"我的背包"},
		Template: "背包里的数量不够。\n发送“我的背包”查看已有物品。",
	},
	"error.invalid_bind_token": {
		Key: "error.invalid_bind_token", Description: "绑定码无效或过期",
		Kind: "validation_error", Tone: "克制", NextActions: []string{"生成绑定码"},
		Template: "绑定码无效或已经过期。\n请重新发送“生成绑定码”获得新号码。",
	},
	"error.bind_conflict": {
		Key: "error.bind_conflict", Description: "身份已属于其他账号",
		Kind: "business_rejected", Tone: "克制", NextActions: []string{"解绑身份"},
		Template: "这个身份已经绑定了其他存档。\n请先在原场景发送“解绑身份”。",
	},
}

func Catalog() []Definition {
	result := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		copyDefinition := definition
		copyDefinition.Variables = cloneVariables(definition.Variables)
		copyDefinition.Variants = append([]string(nil), definition.Variants...)
		copyDefinition.ImageCandidates = append([]string(nil), definition.ImageCandidates...)
		copyDefinition.NextActions = append([]string(nil), definition.NextActions...)
		result = append(result, copyDefinition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

// Lookup returns a detached catalog definition for preview and audit tools.
func Lookup(key string) (Definition, bool) {
	definition, ok := definitions[key]
	if !ok {
		return Definition{}, false
	}
	definition.Variables = cloneVariables(definition.Variables)
	definition.Variants = append([]string(nil), definition.Variants...)
	definition.ImageCandidates = append([]string(nil), definition.ImageCandidates...)
	definition.NextActions = append([]string(nil), definition.NextActions...)
	return definition, true
}

func Render(key string, variables map[string]string) (string, error) {
	definition, ok := definitions[key]
	if !ok {
		return "", fmt.Errorf("unknown player message key %q", key)
	}
	values := cloneVariables(definition.Variables)
	for name, value := range variables {
		values[name] = value
	}
	parsed, err := template.New(key).Option("missingkey=error").Parse(definition.Template)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err = parsed.Execute(&output, values); err != nil {
		return "", err
	}
	return output.String(), nil
}

func MustRender(key string, variables map[string]string) string {
	message, err := Render(key, variables)
	if err != nil {
		return "这次操作没有完成，请稍后再试。"
	}
	return message
}

func cloneVariables(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
