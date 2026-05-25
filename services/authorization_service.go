package services

import "contract-manage/models"

type Resource string
type Action string

const (
	ResourceAuditLogs Resource = "audit_logs"
	ResourceUsers     Resource = "users"
	ResourceApprovals Resource = "approvals"
	ResourceWorkflow  Resource = "workflow"
	ResourceStatusChanges Resource = "status_changes"
)

const (
	ActionView   Action = "view"
	ActionExport Action = "export"
	ActionDelete Action = "delete"
	ActionApprove Action = "approve"
	ActionReject  Action = "reject"
	ActionCreate  Action = "create"
)

type AuthorizationService struct{}

func NewAuthorizationService() *AuthorizationService {
	return &AuthorizationService{}
}

func (s *AuthorizationService) CanRole(role string, resource Resource, action Action) (bool, string) {
	switch resource {
	case ResourceAuditLogs:
		return s.canAuditLogs(role, action)
	case ResourceUsers:
		return s.canUsers(role, action)
	case ResourceApprovals:
		return s.canApprovals(role, action)
	case ResourceWorkflow:
		return s.canWorkflow(role, action)
	case ResourceStatusChanges:
		return s.canStatusChanges(role, action)
	default:
		return false, "权限策略未配置，已默认拒绝"
	}
}

func (s *AuthorizationService) canAuditLogs(role string, action Action) (bool, string) {
	switch action {
	case ActionView, ActionExport:
		if role == string(models.RoleAdmin) || role == string(models.RoleAuditAdmin) {
			return true, ""
		}
		return false, "无权限查看或导出审计日志"
	case ActionDelete:
		return false, "审计日志不允许物理删除"
	default:
		return false, "权限动作未配置，已默认拒绝"
	}
}

func (s *AuthorizationService) canUsers(role string, action Action) (bool, string) {
	switch action {
	case ActionView, ActionDelete:
		if role == string(models.RoleAdmin) {
			return true, ""
		}
		return false, "无权限管理用户"
	default:
		return false, "权限动作未配置，已默认拒绝"
	}
}

func (s *AuthorizationService) canApprovals(role string, action Action) (bool, string) {
	switch action {
	case ActionView, ActionApprove:
		if role == string(models.RoleAdmin) || role == string(models.RoleManager) {
			return true, ""
		}
		return false, "无权限处理审批任务"
	default:
		return false, "权限动作未配置，已默认拒绝"
	}
}

func (s *AuthorizationService) canWorkflow(role string, action Action) (bool, string) {
	switch action {
	case ActionCreate:
		if role == string(models.RoleAdmin) || role == string(models.RoleManager) || role == string(models.RoleUser) {
			return true, ""
		}
		return false, "无权限发起工作流审批"
	case ActionView:
		if role == string(models.RoleAdmin) || role == string(models.RoleManager) {
			return true, ""
		}
		return false, "无权限查看工作流审批信息"
	case ActionApprove, ActionReject:
		if role == string(models.RoleAdmin) {
			return true, ""
		}
		return false, "无权限处理工作流审批"
	default:
		return false, "权限动作未配置，已默认拒绝"
	}
}

func (s *AuthorizationService) canStatusChanges(role string, action Action) (bool, string) {
	switch action {
	case ActionView, ActionApprove, ActionReject:
		if role == string(models.RoleAdmin) || role == string(models.RoleManager) {
			return true, ""
		}
		return false, "无权限处理状态变更审批"
	default:
		return false, "权限动作未配置，已默认拒绝"
	}
}
