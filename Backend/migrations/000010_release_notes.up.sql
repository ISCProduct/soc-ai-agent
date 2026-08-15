CREATE TABLE release_notes (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    pr_number INT UNSIGNED NOT NULL,
    title VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL,
    merged_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_release_notes_pr_number (pr_number),
    KEY idx_release_notes_merged_at (merged_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
