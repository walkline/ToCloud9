CREATE TABLE `group_invites` (
    `invited` int unsigned NOT NULL DEFAULT '0',
    `inviter` int unsigned NOT NULL DEFAULT '0',
    `groupId` int unsigned NOT NULL DEFAULT '0',
    `invitedName` varchar(12),
    `inviterName` varchar(12),
    PRIMARY KEY (`invited`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `groups`
    MODIFY COLUMN `guid` INT UNSIGNED NOT NULL AUTO_INCREMENT;
