ALTER TABLE `account_session_lock`
  ADD COLUMN `expires_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD KEY `idx_account_session_lock_expires_at` (`expires_at`),
  DROP COLUMN `gateway_id`;

DROP TABLE IF EXISTS `gateway_session_liveness`;
