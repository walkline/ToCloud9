DROP TABLE IF EXISTS `group_invites`;

ALTER TABLE `groups`
    MODIFY COLUMN `groupId` INT UNSIGNED DEFAULT '0' NOT NULL;

