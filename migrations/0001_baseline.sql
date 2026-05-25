-- 0001_baseline.sql
-- Purpose: establish the first managed baseline for the legacy schema that was previously created by GORM AutoMigrate.
-- Notes:
--   1. This baseline preserves current field semantics for compatibility.
--   2. Monetary columns intentionally remain DOUBLE in this baseline and should be migrated to DECIMAL in a later controlled change set.
--   3. Status fields intentionally remain VARCHAR in this baseline and should be normalized in a later controlled change set.

USE contract_manage;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL,
    checksum VARCHAR(128) NULL,
    installed_by VARCHAR(64) NOT NULL DEFAULT 'manual',
    installed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    notes TEXT NULL,
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NULL,
    hashed_password VARCHAR(200) NOT NULL,
    full_name VARCHAR(100) NULL,
    role VARCHAR(20) NULL DEFAULT 'user',
    department VARCHAR(100) NULL,
    phone VARCHAR(20) NULL,
    is_active TINYINT(1) NULL DEFAULT 1,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username),
    UNIQUE KEY uk_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS roles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL,
    description TEXT NULL,
    permissions TEXT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS customers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(200) NOT NULL,
    type VARCHAR(20) NULL DEFAULT 'customer',
    code VARCHAR(50) NULL,
    contact_person VARCHAR(100) NULL,
    contact_phone VARCHAR(20) NULL,
    contact_email VARCHAR(100) NULL,
    address TEXT NULL,
    credit_rating VARCHAR(20) NULL,
    is_active TINYINT(1) NULL DEFAULT 1,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_customers_code (code),
    KEY idx_customers_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contract_types (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NULL,
    description TEXT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_contract_types_name (name),
    UNIQUE KEY uk_contract_types_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contracts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_no VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    customer_id BIGINT UNSIGNED NULL,
    contract_type_id BIGINT UNSIGNED NULL,
    amount DOUBLE NULL,
    currency VARCHAR(10) NULL DEFAULT 'CNY',
    status VARCHAR(20) NULL DEFAULT 'draft',
    sign_date DATETIME(3) NULL,
    start_date DATETIME(3) NULL,
    end_date DATETIME(3) NULL,
    payment_terms TEXT NULL,
    content TEXT NULL,
    notes TEXT NULL,
    creator_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_contracts_contract_no (contract_no),
    KEY idx_contracts_title (title),
    KEY idx_contracts_customer_id (customer_id),
    KEY idx_contracts_contract_type_id (contract_type_id),
    KEY idx_contracts_creator_id (creator_id),
    CONSTRAINT fk_contracts_customer FOREIGN KEY (customer_id) REFERENCES customers (id),
    CONSTRAINT fk_contracts_contract_type FOREIGN KEY (contract_type_id) REFERENCES contract_types (id),
    CONSTRAINT fk_contracts_creator FOREIGN KEY (creator_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contract_executions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    stage VARCHAR(100) NULL,
    stage_date DATETIME(3) NULL,
    progress DOUBLE NULL DEFAULT 0,
    payment_amount DOUBLE NULL,
    payment_date DATETIME(3) NULL,
    description TEXT NULL,
    operator_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_contract_executions_contract_id (contract_id),
    KEY idx_contract_executions_operator_id (operator_id),
    CONSTRAINT fk_contract_executions_contract FOREIGN KEY (contract_id) REFERENCES contracts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS approval_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    approver_id BIGINT UNSIGNED NULL,
    level INT NULL DEFAULT 1,
    approver_role VARCHAR(20) NULL,
    status VARCHAR(20) NULL DEFAULT 'pending',
    comment TEXT NULL,
    approved_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_approval_records_contract_id (contract_id),
    KEY idx_approval_records_approver_id (approver_id),
    CONSTRAINT fk_approval_records_contract FOREIGN KEY (contract_id) REFERENCES contracts (id),
    CONSTRAINT fk_approval_records_approver FOREIGN KEY (approver_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS documents (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    name VARCHAR(200) NULL,
    file_path VARCHAR(500) NULL,
    file_size BIGINT NULL,
    file_type VARCHAR(50) NULL,
    version VARCHAR(20) NULL DEFAULT '1.0',
    uploader_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_documents_contract_id (contract_id),
    KEY idx_documents_uploader_id (uploader_id),
    CONSTRAINT fk_documents_contract FOREIGN KEY (contract_id) REFERENCES contracts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS contract_lifecycle_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    event_type VARCHAR(50) NULL,
    from_status VARCHAR(50) NULL,
    to_status VARCHAR(50) NULL,
    amount DOUBLE NULL,
    description TEXT NULL,
    operator_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_contract_lifecycle_events_contract_id (contract_id),
    KEY idx_contract_lifecycle_events_operator_id (operator_id),
    CONSTRAINT fk_contract_lifecycle_events_contract FOREIGN KEY (contract_id) REFERENCES contracts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS status_change_requests (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    from_status VARCHAR(50) NULL,
    to_status VARCHAR(50) NULL,
    reason TEXT NULL,
    requester_id BIGINT UNSIGNED NULL,
    approver_id BIGINT UNSIGNED NULL,
    status VARCHAR(20) NULL DEFAULT 'pending',
    comment TEXT NULL,
    approved_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_status_change_requests_contract_id (contract_id),
    KEY idx_status_change_requests_requester_id (requester_id),
    KEY idx_status_change_requests_approver_id (approver_id),
    CONSTRAINT fk_status_change_requests_contract FOREIGN KEY (contract_id) REFERENCES contracts (id),
    CONSTRAINT fk_status_change_requests_requester FOREIGN KEY (requester_id) REFERENCES users (id),
    CONSTRAINT fk_status_change_requests_approver FOREIGN KEY (approver_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reminders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NULL,
    type VARCHAR(50) NULL,
    reminder_date DATETIME(3) NULL,
    days_before INT NULL,
    is_sent TINYINT(1) NULL DEFAULT 0,
    sent_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_reminders_contract_id (contract_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NULL,
    username VARCHAR(100) NULL,
    action VARCHAR(100) NULL,
    module VARCHAR(50) NULL,
    method VARCHAR(20) NULL,
    path VARCHAR(255) NULL,
    ip_address VARCHAR(50) NULL,
    user_agent TEXT NULL,
    request TEXT NULL,
    response TEXT NULL,
    status_code INT NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_audit_logs_user_id (user_id),
    CONSTRAINT fk_audit_logs_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS approval_workflows (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    contract_id BIGINT UNSIGNED NOT NULL,
    current_level INT NULL DEFAULT 1,
    max_level INT NULL DEFAULT 2,
    status VARCHAR(20) NULL DEFAULT 'pending',
    creator_role VARCHAR(20) NOT NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_approval_workflows_contract_id (contract_id),
    KEY idx_approval_workflows_status (status),
    CONSTRAINT fk_approval_workflows_contract FOREIGN KEY (contract_id) REFERENCES contracts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS workflow_approvals (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    workflow_id BIGINT UNSIGNED NOT NULL,
    contract_id BIGINT UNSIGNED NOT NULL,
    approver_id BIGINT UNSIGNED NULL,
    approver_role VARCHAR(20) NOT NULL,
    level INT NOT NULL,
    status VARCHAR(20) NULL DEFAULT 'pending',
    comment TEXT NULL,
    approved_at DATETIME(3) NULL,
    created_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    KEY idx_workflow_approvals_workflow_id (workflow_id),
    KEY idx_workflow_approvals_contract_id (contract_id),
    KEY idx_workflow_approvals_approver_id (approver_id),
    CONSTRAINT fk_workflow_approvals_workflow FOREIGN KEY (workflow_id) REFERENCES approval_workflows (id),
    CONSTRAINT fk_workflow_approvals_approver FOREIGN KEY (approver_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO schema_migrations (version, description, checksum, installed_by, notes)
VALUES (
    '0001_baseline',
    'Initial managed baseline for legacy contract management schema',
    NULL,
    'manual',
    'Bootstrapped from legacy AutoMigrate-managed schema'
)
ON DUPLICATE KEY UPDATE
    description = VALUES(description),
    notes = VALUES(notes);
