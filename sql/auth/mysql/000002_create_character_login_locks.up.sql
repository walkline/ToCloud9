CREATE TABLE IF NOT EXISTS `character_login_lock` (
  `realm_id` INT UNSIGNED NOT NULL,
  `account_id` INT UNSIGNED NOT NULL,
  `character_guid` BIGINT UNSIGNED NOT NULL,
  `gateway_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (`realm_id`, `account_id`),
  UNIQUE KEY `uq_character_login_lock_character` (`realm_id`, `character_guid`),
  KEY `idx_character_login_lock_gateway` (`realm_id`, `gateway_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
