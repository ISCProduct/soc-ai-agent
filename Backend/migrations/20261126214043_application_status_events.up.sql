-- 選考ステータスの遷移履歴。変更者はロールだけを残す（SOCAIAGENT-673）。
CREATE TABLE application_status_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    application_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    from_status VARCHAR(50) NOT NULL DEFAULT '',
    to_status VARCHAR(50) NOT NULL,
    actor_role VARCHAR(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_application_status_events_application (application_id, id),
    KEY idx_application_status_events_user (user_id)
);
