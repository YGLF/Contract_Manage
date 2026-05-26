//go:build operational_scripts
// +build operational_scripts

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	// 1. 登录获取token
	loginData := map[string]string{
		"username": "admin",
		"password": "admin@123456",
	}
	body, _ := json.Marshal(loginData)
	resp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("登录请求失败:", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("1. 登录响应: %d\n", resp.StatusCode)
	fmt.Printf("   内容: %s\n", string(respBody))

	if resp.StatusCode != 200 {
		return
	}

	var loginResult map[string]interface{}
	json.Unmarshal(respBody, &loginResult)
	token, ok := loginResult["token"].(string)
	if !ok {
		fmt.Println("无法获取token")
		return
	}

	fmt.Println("2. 登录成功, token:", token[:30]+"...")

	// 2. 获取待审批列表
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/workflow/pending", nil)
	req.Header.Add("Authorization", "Bearer "+token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("获取待审批失败:", err)
		return
	}
	defer resp2.Body.Close()

	respBody2, _ := io.ReadAll(resp2.Body)
	fmt.Printf("3. 待审批响应: %d\n", resp2.StatusCode)
	fmt.Printf("   内容: %s\n", string(respBody2))
}
