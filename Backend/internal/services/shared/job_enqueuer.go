package shared

// JobEnqueuer は永続ジョブキューへの投入面（#617）。未設定時は従来どおり同期/go func。
// auth・interview 双方から利用される共有インターフェース。
type JobEnqueuer interface {
	EnqueueEmailVerification(userID uint, email, name, token, appURL string) error
	EnqueueEmailReVerification(userID uint, email, name, token, appURL string) error
	EnqueueEmailRegistration(email, token string) error
	EnqueueEmailPasswordReset(email, token, appURL string) error
	EnqueueInterviewReport(sessionID uint) error
}
