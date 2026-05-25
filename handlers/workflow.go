package handlers

import (
	"contract-manage/middleware"
	"contract-manage/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WorkflowHandler struct {
	workflowService *services.WorkflowService
}

func NewWorkflowHandler(db *gorm.DB) *WorkflowHandler {
	return &WorkflowHandler{
		workflowService: services.NewWorkflowService(db),
	}
}

func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	contractID, err := strconv.ParseUint(c.Param("contract_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid contract ID"})
		return
	}

	workflow, err := h.workflowService.GetWorkflowByContractID(contractID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	c.JSON(http.StatusOK, workflow)
}

func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	var input struct {
		ContractID uint64 `json:"contract_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	role, exists := middleware.GetCurrentUserRole(c)
	if !exists || role == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	workflow, err := h.workflowService.CreateWorkflow(input.ContractID, role, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, workflow)
}

func (h *WorkflowHandler) Approve(c *gin.Context) {
	var input struct {
		WorkflowID uint64 `json:"workflow_id" binding:"required"`
		Level      int    `json:"level" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.workflowService.Approve(input.WorkflowID, input.Level, uint64(userID), input.Comment)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Approved successfully"})
}

func (h *WorkflowHandler) Reject(c *gin.Context) {
	var input struct {
		WorkflowID uint64 `json:"workflow_id" binding:"required"`
		Level      int    `json:"level" binding:"required"`
		Comment    string `json:"comment" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := middleware.GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.workflowService.Reject(input.WorkflowID, input.Level, uint64(userID), input.Comment)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rejected successfully"})
}

func (h *WorkflowHandler) GetMyPendingApproval(c *gin.Context) {
	role, exists := middleware.GetCurrentUserRole(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
		return
	}

	approvals, err := h.workflowService.GetPendingApprovals(role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pending approvals"})
		return
	}

	c.JSON(http.StatusOK, approvals)
}
