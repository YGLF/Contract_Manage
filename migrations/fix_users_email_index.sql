-- 修复 users 表 email 字段索引问题
-- 将 email 的唯一索引改为普通索引，允许为空/重复

-- 删除 email 的唯一索引
ALTER TABLE users DROP INDEX idx_users_email;

-- 创建普通索引（可选，用于查询优化）
ALTER TABLE users ADD INDEX idx_users_email (email);