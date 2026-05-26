//go:build operational_scripts
// +build operational_scripts

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	baseURL := "http://localhost:8080/api"

	fmt.Println("=== 模拟审批流程测试 ===\n")

	// 1. 销售总监登录
	fmt.Println("1. 销售总监登录...")
	token := login(baseURL, "sales_director", "123456")
	if token == "" {
		fmt.Println("   ❌ 登录失败")
		return
	}
	fmt.Println("   ✓ 登录成功")

	// 2. 获取待审批列表
	fmt.Println("\n2. 获取销售总监待审批列表...")
	approvals := getApprovals(baseURL, token)
	fmt.Printf("   ✓ 找到 %d 条待审批\n", len(approvals))
	for _, a := range approvals {
		fmt.Printf("      - %s (%s) 级别: %d\n", a["contract_no"], a["title"], a["level"])
	}

	if len(approvals) == 0 {
		fmt.Println("   ❌ 没有待审批记录")
		return
	}

	// 3. 销售总监同意第一个
	approval := approvals[0]
	workflowID := approval["workflow_id"].(float64)
	level := approval["level"].(float64)
	contractTitle := approval["title"].(string)

	fmt.Printf("\n3. 销售总监同意合同: %s\n", contractTitle)
	result := approve(baseURL, token, uint64(workflowID), int(level), "同意提交审批")
	if result {
		fmt.Println("   ✓ 审批通过")
	} else {
		fmt.Println("   ❌ 审批失败")
	}

	// 4. 重新获取待审批（应该是技术总监级别）
	fmt.Println("\n4. 重新获取待审批...")
	approvals = getApprovals(baseURL, token)
	fmt.Printf("   ✓ 剩余 %d 条待审批\n", len(approvals))

	// 5. 模拟技术总监登录
	fmt.Println("\n5. 技术总监登录...")
	token = login(baseURL, "tech_director", "123456")
	if token == "" {
		fmt.Println("   ❌ 登录失败")
		return
	}
	fmt.Println("   ✓ 登录成功")

	// 6. 获取技术总监待审批
	fmt.Println("\n6. 获取技术总监待审批...")
	approvals = getApprovals(baseURL, token)
	fmt.Printf("   ✓ 找到 %d 条待审批\n", len(approvals))
	for _, a := range approvals {
		fmt.Printf("      - %s (%s) 级别: %d\n", a["contract_no"], a["title"], a["level"])
	}

	if len(approvals) > 0 {
		approval = approvals[0]
		workflowID = approval["workflow_id"].(float64)
		level = approval["level"].(float64)
		contractTitle = approval["title"].(string)

		fmt.Printf("\n7. 技术总监同意合同: %s\n", contractTitle)
		result = approve(baseURL, token, uint64(workflowID), int(level), "技术方案可行，同意")
		if result {
			fmt.Println("   ✓ 审批通过")
		} else {
			fmt.Println("   ❌ 审批失败")
		}
	}

	// 8. 财务总监登录
	fmt.Println("\n8. 财务总监登录...")
	token = login(baseURL, "finance_director", "123456")
	if token == "" {
		fmt.Println("   ❌ 登录失败")
		return
	}
	fmt.Println("   ✓ 登录成功")

	// 9. 获取财务总监待审批
	fmt.Println("\n9. 获取财务总监待审批...")
	approvals = getApprovals(baseURL, token)
	fmt.Printf("   ✓ 找到 %d 条待审批\n", len(approvals))
	for _, a := range approvals {
		fmt.Printf("      - %s (%s) 级别: %d\n", a["contract_no"], a["title"], a["level"])
	}

	if len(approvals) > 0 {
		approval = approvals[0]
		workflowID = approval["workflow_id"].(float64)
		level = approval["level"].(float64)
		contractTitle = approval["title"].(string)

		fmt.Printf("\n10. 财务总监同意合同: %s\n", contractTitle)
		result = approve(baseURL, token, uint64(workflowID), int(level), "财务审核通过，同意归档")
		if result {
			fmt.Println("    ✓ 审批通过，合同已归档")
		} else {
			fmt.Println("    ❌ 审批失败")
		}
	}

	fmt.Println("\n=== 测试完成 ===")
}

func login(baseURL, username, password string) string {
	data := map[string]string{
		"username": username,
		"password": password,
	}
	body, _ := json.Marshal(data)

	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if token, ok := result["token"].(string); ok {
		return token
	}
	return ""
}

func getApprovals(baseURL, token string) []map[string]interface{} {
	req, _ := http.NewRequest("GET", baseURL+"/workflow/pending", nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func approve(baseURL, token string, workflowID uint64, level int, comment string) bool {
	data := map[string]interface{}{
		"workflow_id": workflowID,
		"level":       level,
		"comment":     comment,
	}
	body, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST", baseURL+"/workflow/approve", bytes.NewBuffer(body))
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("   响应: %d %s\n", resp.StatusCode, strings.Trim(string(respBody), "\n"))

	return resp.StatusCode == 200
}
