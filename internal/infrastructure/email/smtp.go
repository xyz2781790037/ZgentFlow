package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// SMTPSender sends verification messages through an SMTP server supporting
// STARTTLS, including smtp.qq.com:587.
type SMTPSender struct {
	host     string
	port     string
	username string
	password string
	from     string
	fromName string
}

func NewSMTPSender() interfaces.VerificationEmailSender {
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	return &SMTPSender{
		host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		port:     port,
		username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		fromName: strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
	}
}

func (s *SMTPSender) SendVerificationCode(ctx context.Context, to, code, purpose string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.host == "" || s.username == "" || s.password == "" || s.from == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}
	fromAddress, err := mail.ParseAddress(s.from)
	if err != nil {
		return fmt.Errorf("invalid SMTP_FROM: %w", err)
	}
	toAddress, err := mail.ParseAddress(to)
	if err != nil || !strings.EqualFold(toAddress.Address, to) {
		return fmt.Errorf("invalid recipient address")
	}

	action := "完成操作"
	if purpose == "register" {
		action = "完成注册"
	} else if purpose == "email_login" {
		action = "登录 ZgentFlow"
	}
	subject := mime.BEncoding.Encode("UTF-8", "ZgentFlow 邮箱验证码")
	fromName := strings.NewReplacer("\r", "", "\n", "").Replace(s.fromName)
	fromHeader := fromAddress.Address
	if fromName != "" {
		fromHeader = mime.BEncoding.Encode("UTF-8", fromName) + " <" + fromAddress.Address + ">"
	}
	body := fmt.Sprintf("您的验证码是：%s\r\n\r\n请在 5 分钟内使用该验证码%s。若非本人操作，请忽略此邮件。", code, action)
	message := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + toAddress.Address,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")

	if err := s.sendMail(ctx, fromAddress.Address, toAddress.Address, []byte(message)); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}

func (s *SMTPSender) sendMail(ctx context.Context, from, to string, message []byte) error {
	address := net.JoinHostPort(s.host, s.port)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))

	client, err := smtp.NewClient(connection, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if supported, _ := client.Extension("STARTTLS"); !supported {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := client.Quit(); err != nil && err != io.EOF {
		return err
	}
	return nil
}
