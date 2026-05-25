package middleware

import (
	"contract-manage/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

var defaultAuthorizationService = services.NewAuthorizationService()

func RequirePermission(resource services.Resource, action services.Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizeRequest(c, resource, action) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func AuthorizeRequest(c *gin.Context, resource services.Resource, action services.Action) bool {
	role, exists := GetCurrentUserRole(c)
	if !exists || role == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证或登录状态已失效"})
		return false
	}

	allowed, reason := defaultAuthorizationService.CanRole(role, resource, action)
	if allowed {
		return true
	}

	if reason == "" {
		reason = "无权限执行该操作"
	}

	c.JSON(http.StatusForbidden, gin.H{
		"error":    reason,
		"resource": resource,
		"action":   action,
	})
	return false
}
