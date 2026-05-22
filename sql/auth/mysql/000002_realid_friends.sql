CREATE TABLE IF NOT EXISTS tc9_realid_friends (
  account_id_low INT UNSIGNED NOT NULL,
  account_id_high INT UNSIGNED NOT NULL,
  requester_account_id INT UNSIGNED NOT NULL,
  status TINYINT UNSIGNED NOT NULL DEFAULT 1,
  note_low VARCHAR(48) NOT NULL DEFAULT '',
  note_high VARCHAR(48) NOT NULL DEFAULT '',
  created_at INT UNSIGNED NOT NULL,
  updated_at INT UNSIGNED NOT NULL,
  PRIMARY KEY (account_id_low, account_id_high),
  KEY idx_tc9_realid_friends_low_status (account_id_low, status),
  KEY idx_tc9_realid_friends_high_status (account_id_high, status),
  KEY idx_tc9_realid_friends_requester (requester_account_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
