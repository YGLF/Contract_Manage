-- 创建通知表
CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    contract_id BIGINT UNSIGNED NOT NULL,
    workflow_id BIGINT UNSIGNED NOT NULL,
    target_role VARCHAR(20),
    notification_type VARCHAR(50),
    title VARCHAR(200),
    content TEXT,
    is_read TINYINT(1) DEFAULT 0,
    read_at DATETIME,
    created_at DATETIME,
    INDEX idx_user_id (user_id),
    INDEX idx_contract_id (contract_id),
    INDEX idx_is_read (is_read)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
