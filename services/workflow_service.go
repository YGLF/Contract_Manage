package services

import (
	"contract-manage/models"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowService struct {
	db              *gorm.DB
	contractService *ContractService
}

func NewWorkflowService(db *gorm.DB) *WorkflowService {
	return &WorkflowService{
		db:              db,
		contractService: NewContractService(),
	}
}

func (s *WorkflowService) CreateWorkflow(contractID uint64, creatorRole string, operatorID uint) (*models.ApprovalWorkflow, error) {
	workflow := &models.ApprovalWorkflow{}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		contract, err := s.contractService.getContractForUpdate(tx, uint(contractID))
		if err != nil {
			return err
		}
		if contract.Status != models.StatusDraft && contract.Status != models.StatusPending {
			return fmt.Errorf("当前合同状态%s不允许创建审批流", contract.Status)
		}

		workflow = &models.ApprovalWorkflow{
			ContractID:   contractID,
			CurrentLevel: 1,
			MaxLevel:     2,
			Status:       models.WorkflowStatusPending,
			CreatorRole:  creatorRole,
		}

		if err := tx.Create(workflow).Error; err != nil {
			return err
		}

		approvers := []models.WorkflowApproval{
			{
				WorkflowID:   workflow.ID,
				ContractID:   contractID,
				ApproverRole: string(models.RoleManager),
				Level:        1,
				Status:       models.WorkflowStatusPending,
			},
			{
				WorkflowID:   workflow.ID,
				ContractID:   contractID,
				ApproverRole: string(models.RoleAdmin),
				Level:        2,
				Status:       models.WorkflowStatusPending,
			},
		}

		if err := tx.Create(&approvers).Error; err != nil {
			return err
		}

		if contract.Status == models.StatusDraft {
			_, err = s.contractService.transitionContractStatusTx(tx, uint(contractID), models.StatusPending, operatorID, contractStatusTransitionOptions{
				EventType:   models.LifecycleSubmitted,
				Description: "合同提交审批",
			})
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (s *WorkflowService) GetWorkflowByContractID(contractID uint64) (*models.ApprovalWorkflow, error) {
	var workflow models.ApprovalWorkflow
	if err := s.db.Preload("Approvals.Approver").Where("contract_id = ?", contractID).First(&workflow).Error; err != nil {
		return nil, err
	}
	return &workflow, nil
}

func (s *WorkflowService) Approve(workflowID uint64, level int, approverID uint64, approverRole string, comment string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var workflow models.ApprovalWorkflow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&workflow, workflowID).Error; err != nil {
			return err
		}
		if workflow.CurrentLevel != level || workflow.Status != models.WorkflowStatusPending {
			return fmt.Errorf("当前审批流不允许处理第%d级审批", level)
		}

		var approval models.WorkflowApproval
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workflow_id = ? AND level = ?", workflowID, level).
			First(&approval).Error; err != nil {
			return err
		}

		if approval.Status != models.WorkflowStatusPending {
			return gorm.ErrRecordNotFound
		}
		if approval.ApproverRole != approverRole {
			return fmt.Errorf("当前角色%s无权处理第%d级审批，需角色%s", approverRole, level, approval.ApproverRole)
		}

		now := time.Now()
		if err := tx.Model(&approval).Updates(map[string]interface{}{
			"status":      models.WorkflowStatusApproved,
			"approver_id": approverID,
			"comment":     comment,
			"approved_at": now,
		}).Error; err != nil {
			return err
		}

		if level >= workflow.MaxLevel {
			contract, err := s.contractService.getContractForUpdate(tx, uint(workflow.ContractID))
			if err != nil {
				return err
			}
			if contract.Status != models.StatusPending {
				return fmt.Errorf("仅待审批状态合同可完成审批流，当前状态为%s", contract.Status)
			}

			if err := tx.Model(&workflow).Update("status", models.WorkflowStatusCompleted).Error; err != nil {
				return err
			}

			_, err = s.contractService.transitionContractStatusTx(tx, uint(workflow.ContractID), models.StatusActive, uint(approverID), contractStatusTransitionOptions{
				EventType:   models.LifecycleApproved,
				Description: "审批通过",
			})
			return err
		}

		return tx.Model(&workflow).Updates(map[string]interface{}{
			"current_level": level + 1,
			"status":        models.WorkflowStatusPending,
		}).Error
	})
}

func (s *WorkflowService) Reject(workflowID uint64, level int, approverID uint64, approverRole string, comment string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var workflow models.ApprovalWorkflow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&workflow, workflowID).Error; err != nil {
			return err
		}
		if workflow.CurrentLevel != level || workflow.Status != models.WorkflowStatusPending {
			return fmt.Errorf("当前审批流不允许处理第%d级审批", level)
		}

		var approval models.WorkflowApproval
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workflow_id = ? AND level = ?", workflowID, level).
			First(&approval).Error; err != nil {
			return err
		}

		if approval.Status != models.WorkflowStatusPending {
			return gorm.ErrRecordNotFound
		}
		if approval.ApproverRole != approverRole {
			return fmt.Errorf("当前角色%s无权处理第%d级审批，需角色%s", approverRole, level, approval.ApproverRole)
		}

		now := time.Now()
		if err := tx.Model(&approval).Updates(map[string]interface{}{
			"status":      models.WorkflowStatusRejected,
			"approver_id": approverID,
			"comment":     comment,
			"approved_at": now,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&workflow).Update("status", models.WorkflowStatusRejected).Error; err != nil {
			return err
		}

		contract, err := s.contractService.getContractForUpdate(tx, uint(workflow.ContractID))
		if err != nil {
			return err
		}
		if contract.Status != models.StatusPending {
			return fmt.Errorf("仅待审批状态合同可拒绝审批流，当前状态为%s", contract.Status)
		}

		_, err = s.contractService.transitionContractStatusTx(tx, uint(workflow.ContractID), models.StatusDraft, uint(approverID), contractStatusTransitionOptions{
			EventType:   models.LifecycleRejected,
			Description: "审批拒绝",
		})
		return err
	})
}

func (s *WorkflowService) GetPendingApprovals(role string) ([]models.WorkflowApproval, error) {
	var approvals []models.WorkflowApproval
	if err := s.db.Preload("Approver").Preload("Workflow").
		Joins("JOIN approval_workflows ON approval_workflows.id = workflow_approvals.workflow_id").
		Where("workflow_approvals.approver_role = ?", role).
		Where("workflow_approvals.level = approval_workflows.current_level").
		Where("approval_workflows.status = ?", models.WorkflowStatusPending).
		Where("workflow_approvals.status = ?", models.WorkflowStatusPending).
		Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}
