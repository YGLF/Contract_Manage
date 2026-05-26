//go:build operational_scripts
// +build operational_scripts

package main

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := mustOperationalDSN()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("连接数据库失败:", err)
	}

	// 直接执行SQL
	sqlQuery := `SELECT wa.id, wa.workflow_id, wa.contract_id, wa.approver_id, wa.approver_role, wa.level, wa.status, wa.comment, wa.hash, wa.approved_at, wa.created_at 
		FROM workflow_approvals wa
		JOIN approval_workflows w ON w.id = wa.workflow_id
		WHERE wa.status = 'pending' AND w.status = 'pending'`

	rows, err := db.Raw(sqlQuery).Rows()
	if err != nil {
		fmt.Println("SQL执行失败:", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, workflowID, contractID, level int
		var approverRole, status, comment, hash string
		var approverID, approvedAt, createdAt interface{}
		rows.Scan(&id, &workflowID, &contractID, &approverID, &approverRole, &level, &status, &comment, &hash, &approvedAt, &createdAt)
		fmt.Printf("记录#%d: id=%d, workflow_id=%d, contract_id=%d, approver_role=%s, level=%d, status=%s\n",
			count+1, id, workflowID, contractID, approverRole, level, status)
		count++
	}
	fmt.Printf("\n共找到 %d 条记录\n", count)
}
