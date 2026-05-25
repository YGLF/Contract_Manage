CREATE DATABASE IF NOT EXISTS contract_manage CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE contract_manage;

-- Bootstrap only: this file provisions the database and account.
-- Development/test may still rely on application-side AutoMigrate when APP_ENV is local/test.
-- Protected environments must apply SQL files under ./migrations in order, starting from 0001_baseline.sql.

CREATE USER IF NOT EXISTS 'contract_user'@'%' IDENTIFIED BY 'contract123';
GRANT ALL PRIVILEGES ON contract_manage.* TO 'contract_user'@'%';
FLUSH PRIVILEGES;
