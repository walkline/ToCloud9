SET @tc9_has_petition_sign_id_idx := (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition_sign'
      AND INDEX_NAME = 'idx_petition_id_player'
);
SET @tc9_sql := IF(@tc9_has_petition_sign_id_idx = 1,
    'ALTER TABLE `petition_sign` DROP KEY `idx_petition_id_player`',
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
SET @tc9_sql := IF(@tc9_has_petition_sign_id = 1,
    'ALTER TABLE `petition_sign` DROP COLUMN `petition_id`',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;

SET @tc9_has_petition_id_idx := (
    SELECT COUNT(*)
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition'
      AND INDEX_NAME = 'idx_petition_id'
);
SET @tc9_sql := IF(@tc9_has_petition_id_idx = 1,
    'ALTER TABLE `petition` DROP KEY `idx_petition_id`',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;

SET @tc9_has_petition_id := (
    SELECT COUNT(*)
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'petition'
      AND COLUMN_NAME = 'petition_id'
);
SET @tc9_sql := IF(@tc9_has_petition_id = 1,
    'ALTER TABLE `petition` DROP COLUMN `petition_id`',
    'SELECT 1');
PREPARE tc9_stmt FROM @tc9_sql;
EXECUTE tc9_stmt;
DEALLOCATE PREPARE tc9_stmt;
