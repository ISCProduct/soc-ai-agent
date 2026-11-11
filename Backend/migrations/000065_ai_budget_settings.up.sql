CREATE TABLE ai_budget_settings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    scope VARCHAR(16) NOT NULL,
    scope_id BIGINT UNSIGNED NOT NULL,
    daily_limit_usd DECIMAL(12,6) NOT NULL DEFAULT 0,
    monthly_limit_usd DECIMAL(12,6) NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY idx_ai_budget_settings_scope (scope, scope_id),
    CONSTRAINT chk_ai_budget_settings_scope CHECK (scope IN ('user', 'school')),
    CONSTRAINT chk_ai_budget_settings_limits CHECK (daily_limit_usd >= 0 AND monthly_limit_usd >= 0)
);
