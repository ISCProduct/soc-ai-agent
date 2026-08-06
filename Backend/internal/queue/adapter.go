package queue

import "Backend/internal/services"

// EnqueuerAdapter は queue.Client を services.JobEnqueuer に適合させる（#617）。
type EnqueuerAdapter struct {
	Client *Client
}

func (a *EnqueuerAdapter) EnqueueEmailVerification(userID uint, email, name, token, appURL string) error {
	return a.Client.EnqueueEmailVerification(EmailVerificationPayload{
		UserID: userID, Email: email, Name: name, Token: token, AppURL: appURL,
	})
}

func (a *EnqueuerAdapter) EnqueueEmailReVerification(userID uint, email, name, token, appURL string) error {
	return a.Client.EnqueueEmailReVerification(EmailVerificationPayload{
		UserID: userID, Email: email, Name: name, Token: token, AppURL: appURL,
	})
}

func (a *EnqueuerAdapter) EnqueueEmailRegistration(email, token string) error {
	return a.Client.EnqueueEmailRegistration(EmailRegistrationPayload{Email: email, Token: token})
}

func (a *EnqueuerAdapter) EnqueueEmailPasswordReset(email, token, appURL string) error {
	return a.Client.EnqueueEmailPasswordReset(EmailPasswordResetPayload{Email: email, Token: token, AppURL: appURL})
}

func (a *EnqueuerAdapter) EnqueueInterviewReport(sessionID uint) error {
	return a.Client.EnqueueInterviewReport(sessionID)
}

// コンパイル時チェック
var _ services.JobEnqueuer = (*EnqueuerAdapter)(nil)
