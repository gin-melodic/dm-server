-- 1. Users table
CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'User ID',
  `openid` VARCHAR(128) NOT NULL COMMENT 'WeChat openid, globally unique',
  `unionid` VARCHAR(128) DEFAULT NULL COMMENT 'WeChat unionid, nullable',
  `nickname` VARCHAR(64) DEFAULT NULL COMMENT 'User nickname',
  `avatar_url` VARCHAR(255) DEFAULT NULL COMMENT 'Avatar URL',
  `mobile` VARCHAR(20) DEFAULT NULL COMMENT 'Mobile number',
  `email` VARCHAR(100) DEFAULT NULL COMMENT 'Email address',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created at',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated at',
  `deleted_at` DATETIME DEFAULT NULL COMMENT 'Deleted at, NULL means not deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_openid` (`openid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Users table';

-- insert dev test user
-- INSERT INTO `users` (`openid`, `unionid`, `nickname`, `avatar_url`, `mobile`, `email`) VALUES ('oXXXXXXXXXXXXXXXXXXXXXXX', 'uXXXXXXXXXXXXXXXXXXXXXXX', 'Dev Test User', 'https://example.com/avatar.jpg', '123456789012', '');

-- 2. Dreams table
CREATE TABLE IF NOT EXISTS `dreams` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Dream ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'Owner user ID',
  `title` VARCHAR(128) DEFAULT NULL COMMENT 'Dream title',
  `content` TEXT NOT NULL COMMENT 'Dream content',
  `dream_date` DATE NOT NULL COMMENT 'Date the dream occurred',
  `tags` VARCHAR(255) DEFAULT NULL COMMENT 'Comma-separated tag list',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created at',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated at',
  `deleted_at` DATETIME DEFAULT NULL COMMENT 'Deleted at, NULL means not deleted',
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'Dream status: pending/processing/completed/error',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  CONSTRAINT `fk_dreams_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Dreams table';

-- 3. Analysis sessions table - for persistence only
CREATE TABLE IF NOT EXISTS `analysis_sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Session ID',
  `dream_id` BIGINT UNSIGNED NOT NULL COMMENT 'Associated dream ID',
  `session_uuid` CHAR(36) NOT NULL COMMENT 'Analysis session UUID',
  `analysis_type` VARCHAR(32) NOT NULL DEFAULT 'dream' COMMENT 'Analysis type, e.g. psychology',
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'Status: pending/processing/completed/error',
  `progress` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'Progress percentage',
  `result_summary` TEXT DEFAULT NULL COMMENT 'Analysis summary (stored upon completion)',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created at',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated at',
  `deleted_at` DATETIME DEFAULT NULL COMMENT 'Deleted at, NULL means not deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_uuid` (`session_uuid`),
  KEY `idx_dream_id` (`dream_id`),
  CONSTRAINT `fk_analysis_sessions_dreams` FOREIGN KEY (`dream_id`) REFERENCES `dreams`(`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Analysis sessions table';

-- System configuration table
CREATE TABLE IF NOT EXISTS `system_configs` (
  `key` VARCHAR(64) NOT NULL COMMENT 'Config key',
  `value` TEXT NOT NULL COMMENT 'Config value (JSON or plain text)',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated at',
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='System configuration table';
