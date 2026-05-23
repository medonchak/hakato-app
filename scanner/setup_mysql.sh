#!/bin/bash
set -e

echo "=== UPDATE & INSTALL MYSQL ==="
sudo apt update
sudo apt install -y mysql-server

echo "=== START MYSQL ==="
sudo systemctl enable mysql
sudo systemctl start mysql

echo "=== SECURE INSTALLATION ==="
sudo mysql <<EOF
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY 'rootpass';
FLUSH PRIVILEGES;
EOF

echo "=== CREATE DATABASE AND TABLES ==="
sudo mysql -u root -prootpass <<EOF

CREATE DATABASE IF NOT EXISTS miniapp CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE miniapp;

-- ==========================
-- USERS
-- ==========================
CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    telegram_id BIGINT UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==========================
-- ALERT RULES
-- ==========================
CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ==========================
-- ALERT FILTERS
-- ==========================
CREATE TABLE IF NOT EXISTS alert_filters (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    rule_id BIGINT NOT NULL,
    filter JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

-- ==========================
-- ALERT MATCHES
-- ==========================
CREATE TABLE IF NOT EXISTS alert_matches (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    rule_id BIGINT NOT NULL,
    tx_hash VARCHAR(100) NOT NULL,
    short TEXT,
    details JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

-- ==========================
-- PORTFOLIOS
-- ==========================
CREATE TABLE IF NOT EXISTS portfolios (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    name VARCHAR(255),
    total_invested DOUBLE DEFAULT 0,
    total_pnl DOUBLE DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ==========================
-- TOKENS IN PORTFOLIO
-- ==========================
CREATE TABLE IF NOT EXISTS portfolio_tokens (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    portfolio_id BIGINT NOT NULL,
    contract VARCHAR(64) NOT NULL,
    symbol VARCHAR(32),
    amount DOUBLE NOT NULL,
    invested DOUBLE NOT NULL,
    realized DOUBLE DEFAULT 0,
    buy_price_usd DOUBLE DEFAULT 0,
    current_price_usd DOUBLE DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (portfolio_id) REFERENCES portfolios(id) ON DELETE CASCADE
);

-- ==========================
-- SNAPSHOTS
-- ==========================
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    portfolio_id BIGINT NOT NULL,
    snapshot JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (portfolio_id) REFERENCES portfolios(id) ON DELETE CASCADE
);

EOF

echo "=== DONE ==="
