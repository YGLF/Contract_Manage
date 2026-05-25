package services

import (
	"contract-manage/models"
	"testing"
)

func TestAuthorizationService_AuditLogsPermissions(t *testing.T) {
	svc := NewAuthorizationService()

	tests := []struct {
		name    string
		role    string
		action  Action
		allowed bool
	}{
		{
			name:    "admin can view audit logs",
			role:    string(models.RoleAdmin),
			action:  ActionView,
			allowed: true,
		},
		{
			name:    "audit admin can export audit logs",
			role:    string(models.RoleAuditAdmin),
			action:  ActionExport,
			allowed: true,
		},
		{
			name:    "manager cannot view audit logs",
			role:    string(models.RoleManager),
			action:  ActionView,
			allowed: false,
		},
		{
			name:    "user cannot export audit logs",
			role:    string(models.RoleUser),
			action:  ActionExport,
			allowed: false,
		},
		{
			name:    "delete is denied for admin",
			role:    string(models.RoleAdmin),
			action:  ActionDelete,
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := svc.CanRole(tt.role, ResourceAuditLogs, tt.action)
			if allowed != tt.allowed {
				t.Fatalf("expected allowed=%v, got %v", tt.allowed, allowed)
			}
		})
	}
}

func TestAuthorizationService_DefaultDeny(t *testing.T) {
	svc := NewAuthorizationService()

	allowed, reason := svc.CanRole(string(models.RoleAdmin), Resource("unknown"), ActionView)
	if allowed {
		t.Fatal("expected unknown resource to be denied")
	}
	if reason == "" {
		t.Fatal("expected deny reason for unknown resource")
	}
}

func TestAuthorizationService_UserPermissions(t *testing.T) {
	svc := NewAuthorizationService()

	allowed, _ := svc.CanRole(string(models.RoleAdmin), ResourceUsers, ActionView)
	if !allowed {
		t.Fatal("expected admin to manage user list")
	}

	allowed, _ = svc.CanRole(string(models.RoleUser), ResourceUsers, ActionView)
	if allowed {
		t.Fatal("expected non-admin to be denied user list management")
	}

	allowed, _ = svc.CanRole(string(models.RoleAdmin), ResourceUsers, ActionDelete)
	if !allowed {
		t.Fatal("expected admin to delete users")
	}
}

func TestAuthorizationService_ApprovalPermissions(t *testing.T) {
	svc := NewAuthorizationService()

	allowed, _ := svc.CanRole(string(models.RoleManager), ResourceApprovals, ActionView)
	if !allowed {
		t.Fatal("expected manager to view pending approvals")
	}

	allowed, _ = svc.CanRole(string(models.RoleUser), ResourceApprovals, ActionApprove)
	if allowed {
		t.Fatal("expected user to be denied approval handling")
	}

	allowed, _ = svc.CanRole(string(models.RoleAdmin), ResourceWorkflow, ActionApprove)
	if !allowed {
		t.Fatal("expected admin to approve workflow")
	}

	allowed, _ = svc.CanRole(string(models.RoleManager), ResourceWorkflow, ActionApprove)
	if allowed {
		t.Fatal("expected manager to be denied workflow approval")
	}

	allowed, _ = svc.CanRole(string(models.RoleUser), ResourceWorkflow, ActionCreate)
	if !allowed {
		t.Fatal("expected user to be allowed to initiate workflow")
	}

	allowed, _ = svc.CanRole(string(models.RoleManager), ResourceWorkflow, ActionView)
	if !allowed {
		t.Fatal("expected manager to view workflow information")
	}

	allowed, _ = svc.CanRole(string(models.RoleManager), ResourceStatusChanges, ActionApprove)
	if !allowed {
		t.Fatal("expected manager to approve status change requests")
	}
}
