//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	// 1. 登录获取 token
	loginReq := `{"username":"admin","password":"admin@123456","password_hash":""}`
	resp, err := http.Post("http://localhost:8000/api/auth/login", "application/json", strings.NewReader(loginReq))
	if err != nil {
		fmt.Println("登录请求失败:", err)
		os.Exit(1)
	}
	defer resp.Body.Body.Close()

	body, _ := io.ReadAll(resp.Body.Body)
	fmt.Println("登录响应:", string(body))

	// 提取 token
	var loginResp map[string]interface{}
	// 这里简化处理，实际应该解析 JSON
	token := extractToken(string(body))
	if token == "" {
		fmt.Println("无法获取 token")
		os.Exit(1)
	}

	fmt.Println("获取到的 token:", token[:50]+"...")

	// 2. 调用待审批列表 API
	req, _ := http.NewRequest("GET", "http://localhost:8000/api/workflow/pending", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		fmt.Println("API 请求失败:", err)
		os.Exit(1)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	fmt.Printf("API 响应状态: %d\n", resp2.StatusCode)
	fmt.Println("API 响应:", string(body2))
}

func extractToken(s string) string {
	// 简单提取 access_token
	if idx := strings.Index(s, "access_token"); idx >= 0 {
		sub := s[idx:]
		if i := strings.Index(sub, "\""); i >= 0 {
			sub = sub[i+1:]
			if j := strings.Index(sub, "\""); j >= 0 {
				return sub[:j]
			}
		}
	}
	return ""
}
