CREATE TABLE IF NOT EXISTS `tc9_lfg_dungeon_routes` (
    `realmId` INT UNSIGNED NOT NULL,
    `playerGuid` BIGINT UNSIGNED NOT NULL,
    `dungeonEntry` INT UNSIGNED NOT NULL,
    `mapId` INT UNSIGNED NOT NULL,
    `difficulty` TINYINT UNSIGNED NOT NULL DEFAULT 0,
    `ownerRealmId` INT UNSIGNED NOT NULL DEFAULT 0,
    `isCrossRealm` TINYINT(1) NOT NULL DEFAULT 0,
    `requiresBoundInstance` TINYINT(1) NOT NULL DEFAULT 0,
    `instanceId` INT UNSIGNED NOT NULL DEFAULT 0,
    `createdAt` INT UNSIGNED NOT NULL,
    `updatedAt` INT UNSIGNED NOT NULL,
    PRIMARY KEY (`realmId`, `playerGuid`, `mapId`, `difficulty`),
    INDEX `idx_tc9_lfg_route_player` (`realmId`, `playerGuid`),
    INDEX `idx_tc9_lfg_route_instance` (`instanceId`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
