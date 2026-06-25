CREATE TABLE IF NOT EXISTS `tc9_guild_bank_lock` (
    `guildid` INT UNSIGNED NOT NULL,
    `owner` VARCHAR(128) NOT NULL,
    `expires_at` BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (`guildid`),
    KEY `idx_tc9_guild_bank_lock_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
