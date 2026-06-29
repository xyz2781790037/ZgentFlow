package interfaces

import "context"

// VerificationEmailSender delivers one-time verification codes. Implementations
// own transport credentials; callers never receive or log those credentials.
type VerificationEmailSender interface {
	SendVerificationCode(ctx context.Context, to, code, purpose string) error
}
