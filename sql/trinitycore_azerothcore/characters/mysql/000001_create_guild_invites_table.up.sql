CREATE TABLE IF NOT EXISTS `guild_invites` (
    `charGuid` int unsigned NOT NULL DEFAULT '0',
    `guildId` int unsigned NOT NULL DEFAULT '0',
    PRIMARY KEY (`charGuid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
