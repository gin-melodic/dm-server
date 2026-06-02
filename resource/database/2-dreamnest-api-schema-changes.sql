-- DreamNest API schema changes
-- Adds persistent storage required by the DreamNest frontend endpoints.

ALTER TABLE `dreams`
ADD COLUMN `emotion` VARCHAR(32) NOT NULL DEFAULT 'neutral' COMMENT 'Dream emotion selected by user',
ADD COLUMN `is_favorite` TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Whether this dream is marked as favorite',
ADD COLUMN `confidence_score` DECIMAL(5, 4) NOT NULL DEFAULT 0.8600 COMMENT 'Dream analysis confidence score';

CREATE TABLE IF NOT EXISTS `user_settings` (
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT 'Owner user ID',
    `language` VARCHAR(16) NOT NULL DEFAULT 'zh-CN' COMMENT 'Preferred language',
    `privacy_mode` VARCHAR(32) NOT NULL DEFAULT 'private' COMMENT 'private or cloud_sync',
    `dream_reminder_enabled` TINYINT(1) NOT NULL DEFAULT 0 COMMENT 'Whether dream reminders are enabled',
    `dream_reminder_time` VARCHAR(8) DEFAULT NULL COMMENT 'Daily reminder time, HH:mm',
    `storage_mode` VARCHAR(32) NOT NULL DEFAULT 'local_cache' COMMENT 'local_cache or cloud_sync',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created at',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated at',
    PRIMARY KEY (`user_id`),
    CONSTRAINT `fk_user_settings_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = 'User settings table';

ALTER TABLE `users` DROP INDEX `uk_openid`;

ALTER TABLE `users` DROP INDEX `uk_supabase_uid`;