package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"contract-manage/internal/microservices/approvalworkflow"
	"contract-manage/internal/microservices/archive"
	"contract-manage/internal/microservices/closure"
	"contract-manage/internal/microservices/contract"
	"contract-manage/internal/microservices/document"
	"contract-manage/internal/microservices/notification"
	"contract-manage/internal/microservices/party"
	"contract-manage/internal/microservices/performance"
	"contract-manage/internal/microservices/report"
	"contract-manage/internal/microservices/risk"
	"contract-manage/internal/microservices/searchai"

	"github.com/gin-gonic/gin"
)

func TestContractIntakeWithCounterpartySync(t *testing.T) {
	gin.SetMode(gin.TestMode)

	partySvc := party.New()
	partyRouter := gin.New()
	partySvc.RegisterRoutes(partyRouter.Group("/api/v1"))
	partyServer := httptest.NewServer(partyRouter)
	defer partyServer.Close()

	mustPostJSON(t, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "示例相对方",
		"unified_social_code": "91310000TEST0001X",
		"status":              "active",
	})

	documentSvc := document.New(t.TempDir())
	documentRouter := gin.New()
	documentSvc.RegisterRoutes(documentRouter.Group("/api/v1"))
	documentServer := httptest.NewServer(documentRouter)
	defer documentServer.Close()

	uploadResp := mustUploadFile(t, documentServer.URL+"/api/v1/documents/temp", "example.pdf", []byte("demo contract pdf"))
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d", uploadResp.StatusCode)
	}
	var uploadBody map[string]interface{}
	mustDecodeBody(t, uploadResp, &uploadBody)
	uploadedDoc := uploadBody["data"].(map[string]interface{})
	tempDocumentID := uploadedDoc["id"].(string)

	mustPostJSON(t, documentServer.URL+"/api/v1/documents/commit", map[string]interface{}{
		"temp_document_id": tempDocumentID,
	})

	contractSvc := contract.New()
	contractSvc.SetDocumentServiceURL(documentServer.URL)
	contractSvc.SetPartyServiceURL(partyServer.URL)
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()

	resp := mustPostJSON(t, contractServer.URL+"/api/v1/contracts/intake", map[string]interface{}{
		"title":           "一期测试合同",
		"counterparty_id": "party-0001",
		"document_ids":    []string{tempDocumentID},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	summaryResp := mustGet(t, partyServer.URL+"/api/v1/parties/party-0001/cooperation-summary")
	var summary map[string]interface{}
	mustDecodeBody(t, summaryResp, &summary)
	data := summary["data"].(map[string]interface{})
	if int(data["cooperation_count"].(float64)) != 1 {
		t.Fatalf("expected cooperation_count=1, got %v", data["cooperation_count"])
	}
}

func TestApprovalWorkflowCallbacks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	contractSvc := contract.New()
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()
	mustPostJSON(t, contractServer.URL+"/api/v1/contracts", map[string]interface{}{
		"title":           "审批回调合同",
		"counterparty_id": "party-001",
		"document_ids":    []string{},
	})

	performanceSvc := performance.New()
	performanceRouter := gin.New()
	performanceSvc.RegisterRoutes(performanceRouter.Group("/api/v1"))
	performanceServer := httptest.NewServer(performanceRouter)
	defer performanceServer.Close()

	archiveSvc := archive.New()
	archiveRouter := gin.New()
	archiveSvc.RegisterRoutes(archiveRouter.Group("/api/v1"))
	archiveServer := httptest.NewServer(archiveRouter)
	defer archiveServer.Close()
	mustPostJSON(t, archiveServer.URL+"/api/v1/archive/cases", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"archive_type": "electronic",
	})

	closureSvc := closure.New()
	closureRouter := gin.New()
	closureSvc.RegisterRoutes(closureRouter.Group("/api/v1"))
	closureServer := httptest.NewServer(closureRouter)
	defer closureServer.Close()
	mustPostJSON(t, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0001",
		"request_type":   "close",
		"reason":         "demo closure",
		"requested_by":   "u-admin",
		"risk_checked":   true,
		"performance_ok": true,
		"evidence_ready": true,
	})

	workflowSvc := approvalworkflow.New()
	workflowSvc.SetServiceURLs(contractServer.URL, performanceServer.URL, archiveServer.URL, "")
	workflowRouter := gin.New()
	workflowSvc.RegisterRoutes(workflowRouter.Group("/api/v1"))
	workflowServer := httptest.NewServer(workflowRouter)
	defer workflowServer.Close()

	statusReq := mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"request_type": "status_change",
		"requested_by": "u-admin",
		"payload": map[string]interface{}{
			"status": "closed",
		},
	})
	if statusReq.StatusCode != http.StatusCreated {
		t.Fatalf("expected create approval request 201, got %d", statusReq.StatusCode)
	}
	mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests/apr-0001/approve", map[string]interface{}{
		"approved_by": "u-admin",
	})

	contractResp := mustGet(t, contractServer.URL+"/api/v1/contracts/ctr-0001")
	var contractBody map[string]interface{}
	mustDecodeBody(t, contractResp, &contractBody)
	contractData := contractBody["data"].(map[string]interface{})
	if contractData["status"] != "closed" {
		t.Fatalf("expected contract status closed, got %v", contractData["status"])
	}

	planReq := mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"request_type": "plan_adjustment",
		"requested_by": "u-admin",
		"payload": map[string]interface{}{
			"nodes": []map[string]interface{}{
				{
					"node_name": "验收",
					"node_type": "acceptance",
					"due_date":  "2026-06-30T00:00:00Z",
				},
			},
		},
	})
	if planReq.StatusCode != http.StatusCreated {
		t.Fatalf("expected plan approval request 201, got %d", planReq.StatusCode)
	}
	mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests/apr-0002/approve", map[string]interface{}{
		"approved_by": "u-admin",
	})

	planResp := mustGet(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/plan-versions/latest")
	var planBody map[string]interface{}
	mustDecodeBody(t, planResp, &planBody)
	planData := planBody["data"].(map[string]interface{})
	if int(planData["version"].(float64)) != 1 {
		t.Fatalf("expected latest version 1, got %v", planData["version"])
	}

	archiveReq := mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"resource_id":  "arc-0001",
		"request_type": "archive_borrow",
		"requested_by": "u-admin",
		"payload":      map[string]interface{}{},
	})
	if archiveReq.StatusCode != http.StatusCreated {
		t.Fatalf("expected archive approval request 201, got %d", archiveReq.StatusCode)
	}
	mustPostJSON(t, workflowServer.URL+"/api/v1/approval-requests/apr-0003/approve", map[string]interface{}{
		"approved_by": "u-admin",
	})

	archiveResp := mustGet(t, archiveServer.URL+"/api/v1/archive/cases")
	var archiveBody map[string]interface{}
	mustDecodeBody(t, archiveResp, &archiveBody)
	archiveList := archiveBody["data"].([]interface{})
	firstArchive := archiveList[0].(map[string]interface{})
	if firstArchive["borrow_status"] != "borrowed" {
		t.Fatalf("expected borrow_status borrowed, got %v", firstArchive["borrow_status"])
	}
}

func TestContractUpdateAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	partySvc := party.New()
	partyRouter := gin.New()
	partySvc.RegisterRoutes(partyRouter.Group("/api/v1"))
	partyServer := httptest.NewServer(partyRouter)
	defer partyServer.Close()

	mustPostJSON(t, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "甲方单位",
		"unified_social_code": "91310000TEST0002Y",
		"status":              "active",
	})
	mustPostJSON(t, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "乙方单位",
		"unified_social_code": "91310000TEST0003Z",
		"status":              "active",
	})

	documentSvc := document.New(t.TempDir())
	documentRouter := gin.New()
	documentSvc.RegisterRoutes(documentRouter.Group("/api/v1"))
	documentServer := httptest.NewServer(documentRouter)
	defer documentServer.Close()

	firstUpload := mustUploadFile(t, documentServer.URL+"/api/v1/documents/temp", "first.pdf", []byte("first"))
	var firstBody map[string]interface{}
	mustDecodeBody(t, firstUpload, &firstBody)
	firstDocumentID := firstBody["data"].(map[string]interface{})["id"].(string)
	mustPostJSON(t, documentServer.URL+"/api/v1/documents/commit", map[string]interface{}{
		"temp_document_id": firstDocumentID,
	})

	secondUpload := mustUploadFile(t, documentServer.URL+"/api/v1/documents/temp", "second.pdf", []byte("second"))
	var secondBody map[string]interface{}
	mustDecodeBody(t, secondUpload, &secondBody)
	secondDocumentID := secondBody["data"].(map[string]interface{})["id"].(string)
	mustPostJSON(t, documentServer.URL+"/api/v1/documents/commit", map[string]interface{}{
		"temp_document_id": secondDocumentID,
	})

	contractSvc := contract.New()
	contractSvc.SetDocumentServiceURL(documentServer.URL)
	contractSvc.SetPartyServiceURL(partyServer.URL)
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()

	createResp := mustPostJSON(t, contractServer.URL+"/api/v1/contracts/intake", map[string]interface{}{
		"title":           "原始合同",
		"counterparty_id": "party-0001",
		"document_ids":    []string{firstDocumentID},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected create 201, got %d", createResp.StatusCode)
	}

	updateResp := mustRequestJSON(t, http.MethodPut, contractServer.URL+"/api/v1/contracts/ctr-0001", map[string]interface{}{
		"title":           "更新后的合同",
		"counterparty_id": "party-0002",
		"document_ids":    []string{secondDocumentID},
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected update 200, got %d", updateResp.StatusCode)
	}

	contractResp := mustGet(t, contractServer.URL+"/api/v1/contracts/ctr-0001")
	var contractBody map[string]interface{}
	mustDecodeBody(t, contractResp, &contractBody)
	contractData := contractBody["data"].(map[string]interface{})
	if contractData["title"] != "更新后的合同" {
		t.Fatalf("expected updated title, got %v", contractData["title"])
	}
	if contractData["counterparty_id"] != "party-0002" {
		t.Fatalf("expected updated counterparty, got %v", contractData["counterparty_id"])
	}
	documentIDs := contractData["document_ids"].([]interface{})
	if len(documentIDs) != 1 || documentIDs[0].(string) != secondDocumentID {
		t.Fatalf("expected updated document ids, got %v", documentIDs)
	}

	firstDocumentResp := mustGet(t, documentServer.URL+"/api/v1/documents/temp/"+firstDocumentID)
	var firstDocumentBody map[string]interface{}
	mustDecodeBody(t, firstDocumentResp, &firstDocumentBody)
	firstDocument := firstDocumentBody["data"].(map[string]interface{})
	if firstDocument["status"] != "committed" {
		t.Fatalf("expected first document released to committed, got %v", firstDocument["status"])
	}

	secondDocumentResp := mustGet(t, documentServer.URL+"/api/v1/documents/temp/"+secondDocumentID)
	var secondDocumentBody map[string]interface{}
	mustDecodeBody(t, secondDocumentResp, &secondDocumentBody)
	secondDocument := secondDocumentBody["data"].(map[string]interface{})
	if secondDocument["status"] != "bound" {
		t.Fatalf("expected second document bound, got %v", secondDocument["status"])
	}

	downloadResp := mustGet(t, documentServer.URL+"/api/v1/documents/temp/"+secondDocumentID+"/download")
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("expected download 200, got %d", downloadResp.StatusCode)
	}
	downloadBody, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	_ = downloadResp.Body.Close()
	if string(downloadBody) != "second" {
		t.Fatalf("expected downloaded content 'second', got %q", string(downloadBody))
	}

	deleteResp := mustRequestJSON(t, http.MethodDelete, contractServer.URL+"/api/v1/contracts/ctr-0001", nil)
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected delete 200, got %d", deleteResp.StatusCode)
	}

	finalDocumentResp := mustGet(t, documentServer.URL+"/api/v1/documents/temp/"+secondDocumentID)
	var finalDocumentBody map[string]interface{}
	mustDecodeBody(t, finalDocumentResp, &finalDocumentBody)
	finalDocument := finalDocumentBody["data"].(map[string]interface{})
	if finalDocument["status"] != "committed" {
		t.Fatalf("expected second document released after delete, got %v", finalDocument["status"])
	}

	missingResp := mustGet(t, contractServer.URL+"/api/v1/contracts/ctr-0001")
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected deleted contract 404, got %d", missingResp.StatusCode)
	}
}

func TestRiskNotificationReportAndSearchAI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	notificationSvc := notification.New()
	notificationSvc.SetAutoSendRiskAlerts(true)
	notificationRouter := gin.New()
	notificationSvc.RegisterRoutes(notificationRouter.Group("/api/v1"))
	notificationServer := httptest.NewServer(notificationRouter)
	defer notificationServer.Close()

	riskSvc := risk.New()
	riskSvc.SetNotificationConfig(notificationServer.URL, "u-admin")
	riskRouter := gin.New()
	riskSvc.RegisterRoutes(riskRouter.Group("/api/v1"))
	riskServer := httptest.NewServer(riskRouter)
	defer riskServer.Close()

	contractSvc := contract.New()
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()
	mustPostJSON(t, contractServer.URL+"/api/v1/contracts", map[string]interface{}{
		"title":           "报表合同",
		"counterparty_id": "party-001",
		"document_ids":    []string{},
	})

	approvalSvc := approvalworkflow.New()
	approvalRouter := gin.New()
	approvalSvc.RegisterRoutes(approvalRouter.Group("/api/v1"))
	approvalServer := httptest.NewServer(approvalRouter)
	defer approvalServer.Close()
	mustPostJSON(t, approvalServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"request_type": "status_change",
		"requested_by": "u-admin",
		"payload": map[string]interface{}{
			"status": "closed",
		},
	})

	archiveSvc := archive.New()
	archiveRouter := gin.New()
	archiveSvc.RegisterRoutes(archiveRouter.Group("/api/v1"))
	archiveServer := httptest.NewServer(archiveRouter)
	defer archiveServer.Close()
	mustPostJSON(t, archiveServer.URL+"/api/v1/archive/cases", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"archive_type": "electronic",
	})

	closureSvc := closure.New()
	closureRouter := gin.New()
	closureSvc.RegisterRoutes(closureRouter.Group("/api/v1"))
	closureServer := httptest.NewServer(closureRouter)
	defer closureServer.Close()
	mustPostJSON(t, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0001",
		"request_type":   "close",
		"reason":         "demo closure",
		"requested_by":   "u-admin",
		"risk_checked":   true,
		"performance_ok": true,
		"evidence_ready": true,
	})

	riskResp := mustPostJSON(t, riskServer.URL+"/api/v1/risk/events", map[string]interface{}{
		"contract_id": "ctr-0001",
		"rule_code":   "expiry_warning",
		"severity":    "high",
		"description": "即将到期",
	})
	if riskResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected risk event 201, got %d", riskResp.StatusCode)
	}

	notificationResp := mustGet(t, notificationServer.URL+"/api/v1/notifications/messages")
	var notificationBody map[string]interface{}
	mustDecodeBody(t, notificationResp, &notificationBody)
	notificationList := notificationBody["data"].([]interface{})
	if len(notificationList) == 0 {
		t.Fatal("expected generated notification")
	}

	reportSvc := report.New()
	reportSvc.SetServiceURLs(contractServer.URL, approvalServer.URL, riskServer.URL, archiveServer.URL, closureServer.URL)
	reportRouter := gin.New()
	reportSvc.RegisterRoutes(reportRouter.Group("/api/v1"))
	reportServer := httptest.NewServer(reportRouter)
	defer reportServer.Close()

	dashboardResp := mustGet(t, reportServer.URL+"/api/v1/reports/dashboard")
	var dashboardBody map[string]interface{}
	mustDecodeBody(t, dashboardResp, &dashboardBody)
	dashboardData := dashboardBody["data"].(map[string]interface{})
	overview := dashboardData["overview"].(map[string]interface{})
	if int(overview["open_risks"].(float64)) != 1 {
		t.Fatalf("expected open_risks 1, got %v", overview["open_risks"])
	}

	searchSvc := searchai.New()
	searchSvc.SetDependencies(contractServer.URL, riskServer.URL, reportServer.URL, "", "", archiveServer.URL, "", nil)
	searchRouter := gin.New()
	searchSvc.RegisterRoutes(searchRouter.Group("/api/v1"))
	searchServer := httptest.NewServer(searchRouter)
	defer searchServer.Close()

	searchResp := mustPostJSON(t, searchServer.URL+"/api/v1/search-ai/ask", map[string]interface{}{
		"question": "请汇总当前风险情况",
		"user_id":  "u-admin",
	})
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected search ai 200, got %d", searchResp.StatusCode)
	}
	var searchBody map[string]interface{}
	mustDecodeBody(t, searchResp, &searchBody)
	searchData := searchBody["data"].(map[string]interface{})
	if searchData["intent"] != "risk_overview" {
		t.Fatalf("expected risk_overview, got %v", searchData["intent"])
	}
}

func TestPerformanceAutoRiskAndClosureValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	riskSvc := risk.New()
	riskRouter := gin.New()
	riskSvc.RegisterRoutes(riskRouter.Group("/api/v1"))
	riskServer := httptest.NewServer(riskRouter)
	defer riskServer.Close()

	performanceSvc := performance.New()
	performanceSvc.SetRiskServiceURL(riskServer.URL)
	performanceRouter := gin.New()
	performanceSvc.RegisterRoutes(performanceRouter.Group("/api/v1"))
	performanceServer := httptest.NewServer(performanceRouter)
	defer performanceServer.Close()

	archiveSvc := archive.New()
	archiveRouter := gin.New()
	archiveSvc.RegisterRoutes(archiveRouter.Group("/api/v1"))
	archiveServer := httptest.NewServer(archiveRouter)
	defer archiveServer.Close()

	closureSvc := closure.New()
	closureSvc.SetArchiveURL(archiveServer.URL)
	closureSvc.SetDependencyURLs(performanceServer.URL, riskServer.URL)
	closureRouter := gin.New()
	closureSvc.RegisterRoutes(closureRouter.Group("/api/v1"))
	closureServer := httptest.NewServer(closureRouter)
	defer closureServer.Close()

	planResp := mustPostJSON(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/plan-versions", map[string]interface{}{
		"nodes": []map[string]interface{}{
			{
				"node_name": "交付验收",
				"node_type": "acceptance",
				"due_date":  "2026-06-30T00:00:00Z",
			},
		},
	})
	if planResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected plan version 201, got %d", planResp.StatusCode)
	}
	var planBody map[string]interface{}
	mustDecodeBody(t, planResp, &planBody)
	planID := planBody["data"].(map[string]interface{})["plans"].([]interface{})[0].(map[string]interface{})["id"].(string)

	execResp := mustPostJSON(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/executions", map[string]interface{}{
		"plan_id":     planID,
		"actual_at":   "2026-06-29T08:00:00Z",
		"result":      "exception",
		"remark":      "节点异常",
		"operator_id": "u-admin",
	})
	if execResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected execution 201, got %d", execResp.StatusCode)
	}

	riskResp := mustGet(t, riskServer.URL+"/api/v1/risk/events?contract_id=ctr-0001&status=open")
	var riskBody map[string]interface{}
	mustDecodeBody(t, riskResp, &riskBody)
	riskList := riskBody["data"].([]interface{})
	if len(riskList) != 1 {
		t.Fatalf("expected 1 risk event, got %d", len(riskList))
	}

	dupResp := mustPostJSON(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/executions", map[string]interface{}{
		"plan_id":     planID,
		"actual_at":   "2026-06-29T09:00:00Z",
		"result":      "exception",
		"remark":      "再次异常",
		"operator_id": "u-admin",
	})
	if dupResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected duplicate execution 201, got %d", dupResp.StatusCode)
	}
	riskResp = mustGet(t, riskServer.URL+"/api/v1/risk/events?contract_id=ctr-0001&status=open")
	mustDecodeBody(t, riskResp, &riskBody)
	riskList = riskBody["data"].([]interface{})
	if len(riskList) != 1 {
		t.Fatalf("expected deduped risk count 1, got %d", len(riskList))
	}

	closureResp := mustPostJSON(t, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0001",
		"request_type":   "close",
		"reason":         "申请结案",
		"requested_by":   "u-admin",
		"risk_checked":   true,
		"performance_ok": true,
		"evidence_ready": true,
	})
	if closureResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected closure blocked 409, got %d", closureResp.StatusCode)
	}

	mustPostJSON(t, riskServer.URL+"/api/v1/risk/events/"+riskList[0].(map[string]interface{})["id"].(string)+"/close", map[string]interface{}{})
	mustPostJSON(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/executions", map[string]interface{}{
		"plan_id":     planID,
		"actual_at":   "2026-06-30T10:00:00Z",
		"result":      "completed",
		"remark":      "已完成",
		"operator_id": "u-admin",
	})

	closureResp = mustPostJSON(t, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0001",
		"request_type":   "close",
		"reason":         "申请结案",
		"requested_by":   "u-admin",
		"risk_checked":   false,
		"performance_ok": false,
		"evidence_ready": true,
	})
	if closureResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected closure create 201 after validation pass, got %d", closureResp.StatusCode)
	}
}

func TestCounterpartyCreditRestrictionBlocksIntake(t *testing.T) {
	gin.SetMode(gin.TestMode)

	partySvc := party.New()
	partyRouter := gin.New()
	partySvc.RegisterRoutes(partyRouter.Group("/api/v1"))
	partyServer := httptest.NewServer(partyRouter)
	defer partyServer.Close()

	createPartyResp := mustPostJSON(t, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "高风险相对方",
		"unified_social_code": "91310000TEST9999X",
		"status":              "active",
		"credit_rating":       "D",
		"credit_source":       "manual",
	})
	if createPartyResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected party create 201, got %d", createPartyResp.StatusCode)
	}
	mustPostJSON(t, partyServer.URL+"/api/v1/parties/party-0001/credit-snapshots", map[string]interface{}{
		"rating":      "D",
		"source":      "credit-platform",
		"risk_flag":   "blocked",
		"description": "信用受限",
	})

	documentSvc := document.New(t.TempDir())
	documentRouter := gin.New()
	documentSvc.RegisterRoutes(documentRouter.Group("/api/v1"))
	documentServer := httptest.NewServer(documentRouter)
	defer documentServer.Close()

	uploadResp := mustUploadFile(t, documentServer.URL+"/api/v1/documents/temp", "restricted.pdf", []byte("restricted"))
	var uploadBody map[string]interface{}
	mustDecodeBody(t, uploadResp, &uploadBody)
	documentID := uploadBody["data"].(map[string]interface{})["id"].(string)
	mustPostJSON(t, documentServer.URL+"/api/v1/documents/commit", map[string]interface{}{
		"temp_document_id": documentID,
	})

	contractSvc := contract.New()
	contractSvc.SetDocumentServiceURL(documentServer.URL)
	contractSvc.SetPartyServiceURL(partyServer.URL)
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()

	resp := mustPostJSON(t, contractServer.URL+"/api/v1/contracts/intake", map[string]interface{}{
		"title":           "受限合同",
		"counterparty_id": "party-0001",
		"document_ids":    []string{documentID},
	})
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected intake blocked 502, got %d", resp.StatusCode)
	}
}

func TestDepartmentScopedAccessAcrossServices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	partySvc := party.New()
	partyRouter := gin.New()
	partySvc.RegisterRoutes(partyRouter.Group("/api/v1"))
	partyServer := httptest.NewServer(partyRouter)
	defer partyServer.Close()

	contractSvc := contract.New()
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()

	riskSvc := risk.New()
	riskRouter := gin.New()
	riskSvc.RegisterRoutes(riskRouter.Group("/api/v1"))
	riskServer := httptest.NewServer(riskRouter)
	defer riskServer.Close()

	archiveSvc := archive.New()
	archiveRouter := gin.New()
	archiveSvc.RegisterRoutes(archiveRouter.Group("/api/v1"))
	archiveServer := httptest.NewServer(archiveRouter)
	defer archiveServer.Close()

	reportSvc := report.New()
	reportSvc.SetServiceURLs(contractServer.URL, "", riskServer.URL, archiveServer.URL, "")
	reportRouter := gin.New()
	reportSvc.RegisterRoutes(reportRouter.Group("/api/v1"))
	reportServer := httptest.NewServer(reportRouter)
	defer reportServer.Close()

	approvalSvc := approvalworkflow.New()
	approvalRouter := gin.New()
	approvalSvc.RegisterRoutes(approvalRouter.Group("/api/v1"))
	approvalServer := httptest.NewServer(approvalRouter)
	defer approvalServer.Close()

	performanceSvc := performance.New()
	performanceRouter := gin.New()
	performanceSvc.RegisterRoutes(performanceRouter.Group("/api/v1"))
	performanceServer := httptest.NewServer(performanceRouter)
	defer performanceServer.Close()

	closureSvc := closure.New()
	closureRouter := gin.New()
	closureSvc.RegisterRoutes(closureRouter.Group("/api/v1"))
	closureServer := httptest.NewServer(closureRouter)
	defer closureServer.Close()

	businessHeaders := map[string]string{
		"X-User-Id":          "u-business",
		"X-User-Department":  "business",
		"X-Data-Scope":       "department",
		"X-User-Permissions": "party.write,party.read,contract.create,contract.read,risk.write,risk.read,archive.write,archive.read,report.read,approval.request,approval.read,performance.write,performance.read,closure.request,closure.read",
	}
	legalHeaders := map[string]string{
		"X-User-Id":          "u-legal",
		"X-User-Department":  "legal",
		"X-Data-Scope":       "department",
		"X-User-Permissions": "party.write,party.read,contract.create,contract.read,risk.write,risk.read,archive.write,archive.read,report.read,approval.request,approval.read,performance.write,performance.read,closure.request,closure.read",
	}

	resp := mustRequestJSONWithHeaders(t, http.MethodPost, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "业务相对方",
		"unified_social_code": "91310000ABAC0001X",
		"status":              "active",
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business party create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, partyServer.URL+"/api/v1/parties", map[string]interface{}{
		"name":                "法务相对方",
		"unified_social_code": "91310000ABAC0002X",
		"status":              "active",
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal party create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, contractServer.URL+"/api/v1/contracts", map[string]interface{}{
		"title":           "业务合同",
		"counterparty_id": "party-0001",
		"document_ids":    []string{},
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business contract create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, contractServer.URL+"/api/v1/contracts", map[string]interface{}{
		"title":           "法务合同",
		"counterparty_id": "party-0002",
		"document_ids":    []string{},
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal contract create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, riskServer.URL+"/api/v1/risk/events", map[string]interface{}{
		"contract_id": "ctr-0001",
		"rule_code":   "biz_rule",
		"severity":    "medium",
		"description": "business scope risk",
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business risk create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, riskServer.URL+"/api/v1/risk/events", map[string]interface{}{
		"contract_id": "ctr-0002",
		"rule_code":   "legal_rule",
		"severity":    "high",
		"description": "legal scope risk",
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal risk create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, archiveServer.URL+"/api/v1/archive/cases", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"archive_type": "electronic",
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business archive create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, archiveServer.URL+"/api/v1/archive/cases", map[string]interface{}{
		"contract_id":  "ctr-0002",
		"archive_type": "paper",
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal archive create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, approvalServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"contract_id":  "ctr-0001",
		"request_type": "status_change",
		"requested_by": "u-business",
		"payload": map[string]interface{}{
			"status": "closed",
		},
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business approval create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, approvalServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"contract_id":  "ctr-0002",
		"request_type": "status_change",
		"requested_by": "u-legal",
		"payload": map[string]interface{}{
			"status": "closed",
		},
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal approval create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, performanceServer.URL+"/api/v1/contracts/ctr-0001/plan-versions", map[string]interface{}{
		"nodes": []map[string]interface{}{
			{
				"node_name": "业务节点",
				"node_type": "payment",
				"due_date":  "2026-06-30T00:00:00Z",
			},
		},
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business performance version create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, performanceServer.URL+"/api/v1/contracts/ctr-0002/plan-versions", map[string]interface{}{
		"nodes": []map[string]interface{}{
			{
				"node_name": "法务节点",
				"node_type": "acceptance",
				"due_date":  "2026-07-01T00:00:00Z",
			},
		},
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal performance version create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, performanceServer.URL+"/api/v1/contracts/ctr-0001/executions", map[string]interface{}{
		"plan_id":     "plan-0001",
		"actual_at":   "2026-06-29T08:00:00Z",
		"result":      "completed",
		"remark":      "business done",
		"operator_id": "u-business",
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business execution create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, performanceServer.URL+"/api/v1/contracts/ctr-0002/executions", map[string]interface{}{
		"plan_id":     "plan-0002",
		"actual_at":   "2026-06-29T08:00:00Z",
		"result":      "completed",
		"remark":      "legal done",
		"operator_id": "u-legal",
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal execution create 201, got %d", resp.StatusCode)
	}

	resp = mustRequestJSONWithHeaders(t, http.MethodPost, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0001",
		"request_type":   "close",
		"reason":         "business close",
		"requested_by":   "u-business",
		"risk_checked":   true,
		"performance_ok": true,
		"evidence_ready": true,
	}, businessHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected business closure create 201, got %d", resp.StatusCode)
	}
	resp = mustRequestJSONWithHeaders(t, http.MethodPost, closureServer.URL+"/api/v1/closure/requests", map[string]interface{}{
		"contract_id":    "ctr-0002",
		"request_type":   "close",
		"reason":         "legal close",
		"requested_by":   "u-legal",
		"risk_checked":   true,
		"performance_ok": true,
		"evidence_ready": true,
	}, legalHeaders)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected legal closure create 201, got %d", resp.StatusCode)
	}

	partyListResp := mustGetWithHeaders(t, partyServer.URL+"/api/v1/parties", businessHeaders)
	var partyListBody map[string]interface{}
	mustDecodeBody(t, partyListResp, &partyListBody)
	parties := partyListBody["data"].([]interface{})
	if len(parties) != 1 {
		t.Fatalf("expected 1 scoped party, got %d", len(parties))
	}

	contractListResp := mustGetWithHeaders(t, contractServer.URL+"/api/v1/contracts", businessHeaders)
	var contractListBody map[string]interface{}
	mustDecodeBody(t, contractListResp, &contractListBody)
	contracts := contractListBody["data"].([]interface{})
	if len(contracts) != 1 {
		t.Fatalf("expected 1 scoped contract, got %d", len(contracts))
	}

	riskListResp := mustGetWithHeaders(t, riskServer.URL+"/api/v1/risk/events", businessHeaders)
	var riskListBody map[string]interface{}
	mustDecodeBody(t, riskListResp, &riskListBody)
	risks := riskListBody["data"].([]interface{})
	if len(risks) != 1 {
		t.Fatalf("expected 1 scoped risk, got %d", len(risks))
	}

	archiveListResp := mustGetWithHeaders(t, archiveServer.URL+"/api/v1/archive/cases", businessHeaders)
	var archiveListBody map[string]interface{}
	mustDecodeBody(t, archiveListResp, &archiveListBody)
	archives := archiveListBody["data"].([]interface{})
	if len(archives) != 1 {
		t.Fatalf("expected 1 scoped archive, got %d", len(archives))
	}

	approvalListResp := mustGetWithHeaders(t, approvalServer.URL+"/api/v1/approval-requests", businessHeaders)
	var approvalListBody map[string]interface{}
	mustDecodeBody(t, approvalListResp, &approvalListBody)
	approvals := approvalListBody["data"].([]interface{})
	if len(approvals) != 1 {
		t.Fatalf("expected 1 scoped approval request, got %d", len(approvals))
	}

	performanceListResp := mustGetWithHeaders(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/plans", businessHeaders)
	var performanceListBody map[string]interface{}
	mustDecodeBody(t, performanceListResp, &performanceListBody)
	plans := performanceListBody["data"].([]interface{})
	if len(plans) != 1 {
		t.Fatalf("expected 1 scoped plan, got %d", len(plans))
	}

	executionListResp := mustGetWithHeaders(t, performanceServer.URL+"/api/v1/contracts/ctr-0001/executions", businessHeaders)
	var executionListBody map[string]interface{}
	mustDecodeBody(t, executionListResp, &executionListBody)
	executions := executionListBody["data"].([]interface{})
	if len(executions) != 1 {
		t.Fatalf("expected 1 scoped execution, got %d", len(executions))
	}

	closureListResp := mustGetWithHeaders(t, closureServer.URL+"/api/v1/closure/requests", businessHeaders)
	var closureListBody map[string]interface{}
	mustDecodeBody(t, closureListResp, &closureListBody)
	closures := closureListBody["data"].([]interface{})
	if len(closures) != 1 {
		t.Fatalf("expected 1 scoped closure request, got %d", len(closures))
	}

	forbiddenResp := mustGetWithHeaders(t, contractServer.URL+"/api/v1/contracts/ctr-0002", businessHeaders)
	if forbiddenResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-department contract access 403, got %d", forbiddenResp.StatusCode)
	}

	forbiddenResp = mustGetWithHeaders(t, performanceServer.URL+"/api/v1/contracts/ctr-0002/plan-versions/latest", businessHeaders)
	if forbiddenResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-department performance access 403, got %d", forbiddenResp.StatusCode)
	}

	dashboardResp := mustGetWithHeaders(t, reportServer.URL+"/api/v1/reports/dashboard", businessHeaders)
	var dashboardBody map[string]interface{}
	mustDecodeBody(t, dashboardResp, &dashboardBody)
	overview := dashboardBody["data"].(map[string]interface{})["overview"].(map[string]interface{})
	if int(overview["contract_total"].(float64)) != 1 {
		t.Fatalf("expected scoped contract_total 1, got %v", overview["contract_total"])
	}
	if int(overview["open_risks"].(float64)) != 1 {
		t.Fatalf("expected scoped open_risks 1, got %v", overview["open_risks"])
	}
	if int(overview["archived_contracts"].(float64)) != 1 {
		t.Fatalf("expected scoped archived_contracts 1, got %v", overview["archived_contracts"])
	}
}

func TestReportExportAndDocumentDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	documentSvc := document.New(t.TempDir())
	documentRouter := gin.New()
	documentSvc.RegisterRoutes(documentRouter.Group("/api/v1"))
	documentServer := httptest.NewServer(documentRouter)
	defer documentServer.Close()

	uploadResp := mustUploadFile(t, documentServer.URL+"/api/v1/documents/temp", "export-check.pdf", []byte("export-check"))
	var uploadBody map[string]interface{}
	mustDecodeBody(t, uploadResp, &uploadBody)
	documentID := uploadBody["data"].(map[string]interface{})["id"].(string)
	mustPostJSON(t, documentServer.URL+"/api/v1/documents/commit", map[string]interface{}{
		"temp_document_id": documentID,
	})

	downloadResp := mustGetWithHeaders(t, documentServer.URL+"/api/v1/documents/temp/"+documentID+"/download", map[string]string{
		"X-User-Id":          "u-doc",
		"X-User-Permissions": "document.read",
	})
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("expected document download 200, got %d", downloadResp.StatusCode)
	}
	downloadBytes, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		t.Fatalf("read document download: %v", err)
	}
	_ = downloadResp.Body.Close()
	if string(downloadBytes) != "export-check" {
		t.Fatalf("expected downloaded document content, got %q", string(downloadBytes))
	}

	contractSvc := contract.New()
	contractRouter := gin.New()
	contractSvc.RegisterRoutes(contractRouter.Group("/api/v1"))
	contractServer := httptest.NewServer(contractRouter)
	defer contractServer.Close()
	mustRequestJSONWithHeaders(t, http.MethodPost, contractServer.URL+"/api/v1/contracts", map[string]interface{}{
		"title":           "导出测试合同",
		"counterparty_id": "party-001",
		"document_ids":    []string{},
	}, map[string]string{
		"X-User-Id":          "u-report",
		"X-User-Permissions": "contract.create",
	})

	riskSvc := risk.New()
	riskRouter := gin.New()
	riskSvc.RegisterRoutes(riskRouter.Group("/api/v1"))
	riskServer := httptest.NewServer(riskRouter)
	defer riskServer.Close()

	approvalSvc := approvalworkflow.New()
	approvalRouter := gin.New()
	approvalSvc.RegisterRoutes(approvalRouter.Group("/api/v1"))
	approvalServer := httptest.NewServer(approvalRouter)
	defer approvalServer.Close()

	archiveSvc := archive.New()
	archiveRouter := gin.New()
	archiveSvc.RegisterRoutes(archiveRouter.Group("/api/v1"))
	archiveServer := httptest.NewServer(archiveRouter)
	defer archiveServer.Close()

	closureSvc := closure.New()
	closureRouter := gin.New()
	closureSvc.RegisterRoutes(closureRouter.Group("/api/v1"))
	closureServer := httptest.NewServer(closureRouter)
	defer closureServer.Close()

	reportSvc := report.New()
	reportSvc.SetServiceURLs(contractServer.URL, approvalServer.URL, riskServer.URL, archiveServer.URL, closureServer.URL)
	reportRouter := gin.New()
	reportSvc.RegisterRoutes(reportRouter.Group("/api/v1"))
	reportServer := httptest.NewServer(reportRouter)
	defer reportServer.Close()

	exportHeaders := map[string]string{
		"X-User-Id":          "u-report",
		"X-User-Permissions": "report.export,report.read,approval.request,approval.read,approval.process,contract.read,risk.read,archive.read,closure.read",
	}
	exportResp := mustGetWithHeaders(t, reportServer.URL+"/api/v1/reports/export?view=dashboard", exportHeaders)
	if exportResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected report export without approval 400, got %d", exportResp.StatusCode)
	}

	createApprovalResp := mustRequestJSONWithHeaders(t, http.MethodPost, approvalServer.URL+"/api/v1/approval-requests", map[string]interface{}{
		"request_type": "report_export",
		"requested_by": "u-report",
		"payload": map[string]interface{}{
			"view": "dashboard",
		},
	}, exportHeaders)
	if createApprovalResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected report export approval create 201, got %d", createApprovalResp.StatusCode)
	}
	mustRequestJSONWithHeaders(t, http.MethodPost, approvalServer.URL+"/api/v1/approval-requests/apr-0001/approve", map[string]interface{}{
		"approved_by": "u-report",
	}, exportHeaders)

	exportResp = mustGetWithHeaders(t, reportServer.URL+"/api/v1/reports/export?view=dashboard&approval_request_id=apr-0001", exportHeaders)
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("expected report export 200, got %d", exportResp.StatusCode)
	}
	if contentDisposition := exportResp.Header.Get("Content-Disposition"); contentDisposition == "" {
		t.Fatal("expected export content disposition header")
	}
	var exportBody map[string]interface{}
	mustDecodeBody(t, exportResp, &exportBody)
	if _, ok := exportBody["overview"]; !ok {
		t.Fatalf("expected dashboard export overview, got %v", exportBody)
	}

	secondExportResp := mustGetWithHeaders(t, reportServer.URL+"/api/v1/reports/export?view=dashboard&approval_request_id=apr-0001", exportHeaders)
	if secondExportResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected reused export approval 403, got %d", secondExportResp.StatusCode)
	}
}

func mustPostJSON(t *testing.T, endpoint string, payload map[string]interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("post %s: %v", endpoint, err)
	}
	return resp
}

func mustRequestJSON(t *testing.T, method, endpoint string, payload map[string]interface{}) *http.Response {
	t.Helper()
	return mustRequestJSONWithHeaders(t, method, endpoint, payload, nil)
}

func mustRequestJSONWithHeaders(t *testing.T, method, endpoint string, payload map[string]interface{}, headers map[string]string) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, endpoint, err)
	}
	return resp
}

func mustGet(t *testing.T, endpoint string) *http.Response {
	t.Helper()
	return mustGetWithHeaders(t, endpoint, nil)
}

func mustGetWithHeaders(t *testing.T, endpoint string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create get request %s: %v", endpoint, err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", endpoint, err)
	}
	return resp
}

func mustDecodeBody(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func mustUploadFile(t *testing.T, endpoint, filename string, content []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	return resp
}
