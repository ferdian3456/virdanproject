package util

import (
	"gopkg.in/gomail.v2"
)

func SendEmail(smtpHost string, smtpPort int, senderName string, senderEmail string, senderPassowrd string, receiverEmail string, subject string, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", senderName)
	mailer.SetHeader("To", receiverEmail)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/html", body)

	dialer := gomail.NewDialer(
		smtpHost,
		smtpPort,
		senderEmail,
		senderPassowrd,
	)

	err := dialer.DialAndSend(mailer)
	if err != nil {
		return err
	}

	return nil
}
