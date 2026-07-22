CREATE TABLE IF NOT EXISTS `gateway_session_liveness` (
  `gateway_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`gateway_id`),
  KEY `idx_gateway_session_liveness_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE `account_session_lock`
  ADD COLUMN `gateway_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER `account_id`,
  DROP INDEX `idx_account_session_lock_expires_at`,
  DROP COLUMN `expires_at`;
