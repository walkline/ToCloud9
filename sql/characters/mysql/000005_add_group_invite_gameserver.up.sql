ALTER TABLE `group_invites`
    ADD COLUMN `inviterMapId` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `inviterName`,
    ADD COLUMN `inviterGameServerId` VARCHAR(255) NOT NULL DEFAULT '' AFTER `inviterMapId`;
