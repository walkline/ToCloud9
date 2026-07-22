CREATE TABLE IF NOT EXISTS `account_session_lock` (
  `account_id` INT UNSIGNED NOT NULL,
  `gateway_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `owner_token` CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `gateway_session_liveness` (
  `gateway_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`gateway_id`),
  KEY `idx_gateway_session_liveness_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
