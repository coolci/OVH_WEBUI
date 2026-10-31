package handlers

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseTemplateInputs(t *testing.T) {
	// 模拟 OVH API 返回的 ubuntu2404-withapp_64 详情
	detail := map[string]interface{}{
		"templateName": "ubuntu2404-withapp_64",
		"distribution": "ubuntu2404-withapp",
		"family":       "linux",
		"bitFormat":    float64(64),
		"inputs": []interface{}{
			map[string]interface{}{
				"name":        "hostname",
				"type":        "hostname",
				"description": "Custom hostname",
				"mandatory":   false,
				"default":     "",
				"enum":        []interface{}{},
			},
			map[string]interface{}{
				"name":        "sshKey",
				"type":        "sshPubKey",
				"description": "SSH Public Key",
				"mandatory":   false,
				"default":     "",
				"enum":        []interface{}{},
			},
			map[string]interface{}{
				"name":        "appToInstall",
				"type":        "enum",
				"description": "Application to install",
				"mandatory":   true,
				"default":     "dokploy",
				"enum": []interface{}{
					"coolify",
					"dokploy",
					"wordpress",
				},
			},
		},
	}

	entry := gin.H{
		"templateName": detail["templateName"],
	}

	if inputsRaw, ok := detail["inputs"].([]interface{}); ok && len(inputsRaw) > 0 {
		var questions []gin.H
		for _, inRaw := range inputsRaw {
			if inMap, ok := inRaw.(map[string]interface{}); ok {
				qName, _ := inMap["name"].(string)
				if qName == "" || qName == "hostname" {
					continue
				}
				qDesc, _ := inMap["description"].(string)
				if qDesc == "" {
					qDesc = qName
				}
				qType, _ := inMap["type"].(string)
				if qType == "" {
					qType = "string"
				}
				mandatory, _ := inMap["mandatory"].(bool)
				q := gin.H{
					"name":        qName,
					"description": qDesc,
					"type":        qType,
					"required":    mandatory,
				}
				if defVal, ok := inMap["default"].(string); ok && defVal != "" {
					q["default"] = defVal
				}
				if enumRaw, ok := inMap["enum"].([]interface{}); ok && len(enumRaw) > 0 {
					var enumList []string
					for _, ev := range enumRaw {
						if es, ok := ev.(string); ok && es != "" {
							enumList = append(enumList, es)
						}
					}
					if len(enumList) > 0 {
						q["enum"] = enumList
					}
				}
				questions = append(questions, q)
			}
		}
		if len(questions) > 0 {
			entry["customizeQuestions"] = questions
		}
	}

	qs, ok := entry["customizeQuestions"].([]gin.H)
	if !ok {
		t.Fatalf("expected customizeQuestions to be []gin.H, got %T", entry["customizeQuestions"])
	}
	if len(qs) != 2 {
		t.Fatalf("expected 2 questions (hostname skipped), got %d", len(qs))
	}
	// 校验 sshKey
	if qs[0]["name"] != "sshKey" || qs[0]["required"] != false {
		t.Errorf("unexpected sshKey question: %+v", qs[0])
	}
	// 校验 appToInstall
	appQ := qs[1]
	if appQ["name"] != "appToInstall" || appQ["required"] != true || appQ["type"] != "enum" {
		t.Errorf("unexpected appToInstall question: %+v", appQ)
	}
	enums, ok := appQ["enum"].([]string)
	if !ok || len(enums) != 3 || enums[1] != "dokploy" {
		t.Errorf("unexpected enum list: %+v", appQ["enum"])
	}
	if appQ["default"] != "dokploy" {
		t.Errorf("unexpected default: %v", appQ["default"])
	}
}

