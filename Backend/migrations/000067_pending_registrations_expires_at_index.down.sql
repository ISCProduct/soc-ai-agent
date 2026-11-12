ALTER TABLE `pending_registrations`
  DROP INDEX `idx_pending_registrations_expires_at`,
  ALGORITHM=INPLACE, LOCK=NONE;
