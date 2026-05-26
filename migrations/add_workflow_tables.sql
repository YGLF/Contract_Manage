-- 工作流表结构迁移脚本
-- 用于在已有数据库中添加工作流相关表

-- 1. 创建审批工作流表
CREATE TABLE IF NOT EXISTS `approval_workflows` (
  `id` bigint UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `contract_id` bigint UNSIGNED NOT NULL,
  `creator_id` bigint UNSIGNED,
  `current_level` int DEFAULT 1,
  `max_level` int DEFAULT 3,
  `status` varchar(20) DEFAULT 'pending',
  `creator_role` varchar(20) NOT NULL,
  `hash` varchar(64),
  `created_at` datetime(3),
  `updated_at` datetime(3),
  INDEX `idx_contract_id` (`contract_id`),
  INDEX `idx_creator_id` (`creator_id`),
  INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. 创建工作流审批节点表
CREATE TABLE IF NOT EXISTS `workflow_approvals` (
  `id` bigint UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `workflow_id` bigint UNSIGNED NOT NULL,
  `contract_id` bigint UNSIGNED NOT NULL,
  `approver_id` bigint UNSIGNED,
  `approver_role` varchar(20) NOT NULL,
  `level` int NOT NULL,
  `status` varchar(20) DEFAULT 'pending',
  `comment` text,
  `hash` varchar(64),
  `approved_at` datetime(3),
  `created_at` datetime(3),
  INDEX `idx_workflow_id` (`workflow_id`),
  INDEX `idx_contract_id` (`contract_id`),
  INDEX `idx_approver_role` (`approver_role`),
  INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. 添加合同表的外键关联（可选，如果已有表则跳过）
-- ALTER TABLE `approval_workflows` ADD CONSTRAINT `fk_approval_workflows_contract` FOREIGN KEY (`contract_id`) REFERENCES `contracts`(`id`);
