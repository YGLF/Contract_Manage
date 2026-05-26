//go:build operational_scripts
// +build operational_scripts

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	// 1. 登录获取 token
	loginReq := `{"username":"admin","password":"admin@123456"}`
	resp, err := http.Post("http://localhost:8000/api/auth/login", "application/json", strings.NewReader(loginReq))
	if err != nil {
		fmt.Println("登录请求失败:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("=== 登录响应 ===")
	fmt.Println(string(body))

	// 提取 token
	token := extractToken(string(body))
	if token == "" {
		fmt.Println("无法获取 token")
		return
	}

	fmt.Println("\n=== 调用待审批API ===")

	// 2. 调用待审批列表 API
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8000/api/workflow/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp2, err := client.Do(req)
	if err != nil {
		fmt.Println("API 请求失败:", err)
		return
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	fmt.Printf("状态码: %d\n", resp2.StatusCode)
	fmt.Printf("响应: %s\n", string(body2))
}

func extractToken(s string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return ""
	}
	if token, ok := data["access_token"].(string); ok {
		return token
	}
	return ""
}
