CREATE TABLE IF NOT EXISTS `channels_members` (
  `channelId` INT UNSIGNED NOT NULL,
  `playerGUID` BIGINT UNSIGNED NOT NULL,
  `playerName` VARCHAR(12) NOT NULL,
  `flags` TINYINT UNSIGNED NOT NULL DEFAULT 0,
  `joinedAt` BIGINT NOT NULL,
  PRIMARY KEY (`channelId`, `playerGUID`),
  INDEX `idx_channel` (`channelId`),
  INDEX `idx_player` (`playerGUID`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
