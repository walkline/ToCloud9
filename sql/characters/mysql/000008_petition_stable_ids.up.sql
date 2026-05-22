SET @tc9_has_petition_id := (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition'
      AND COLUMN_NAME = 'petition_id'
);
SET @tc9_sql := IF(@tc9_has_petition_id = 0,
    'ALTER TABLE `petition` ADD COLUMN `petition_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `petitionguid`',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;

SET @tc9_petition_id := (SELECT COALESCE(MAX(`petition_id`), 0) FROM `petition`);
UPDATE `petition`
SET `petition_id` = (@tc9_petition_id := @tc9_petition_id + 1)
WHERE `petition_id` = 0
ORDER BY `ownerguid`, `type`, `petitionguid`;

SET @tc9_has_petition_id_idx := (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition'
      AND INDEX_NAME = 'idx_petition_id'
);
SET @tc9_sql := IF(@tc9_has_petition_id_idx = 0,
    'ALTER TABLE `petition` ADD KEY `idx_petition_id` (`petition_id`)',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;

SET @tc9_has_petition_sign_id := (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition_sign'
      AND COLUMN_NAME = 'petition_id'
);
SET @tc9_sql := IF(@tc9_has_petition_sign_id = 0,
    'ALTER TABLE `petition_sign` ADD COLUMN `petition_id` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `petitionguid`',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;

UPDATE `petition_sign` ps
INNER JOIN `petition` p ON p.`petitionguid` = ps.`petitionguid`
SET ps.`petition_id` = p.`petition_id`
WHERE ps.`petition_id` = 0;

SET @tc9_has_petition_sign_id_idx := (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition_sign'
      AND INDEX_NAME = 'idx_petition_id_player'
);
SET @tc9_sql := IF(@tc9_has_petition_sign_id_idx = 0,
    'ALTER TABLE `petition_sign` ADD KEY `idx_petition_id_player` (`petition_id`, `playerguid`)',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;
