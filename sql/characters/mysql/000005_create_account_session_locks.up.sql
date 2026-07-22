CREATE TABLE IF NOT EXISTS `account_session_lock` (
  `account_id` INT UNSIGNED NOT NULL,
  `owner_token` CHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expires_at` DATETIME(6) NOT NULL,
  PRIMARY KEY (`account_id`),
  KEY `idx_account_session_lock_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
