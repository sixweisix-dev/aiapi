package monitoring

import (
	"fmt"
	"net/smtp"
	"strconv"
)

// MailAlerter 简易 SMTP 邮件推送
type MailAlerter struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	To       string // 收件人(管理员邮箱)
}

func NewMailAlerter(host, port, user, password, from, to string) *MailAlerter {
	if host == "" || user == "" || to == "" {
		return nil
	}
	return &MailAlerter{Host: host, Port: port, User: user, Password: password, From: from, To: to}
}

func (m *MailAlerter) Send(subject, body string) error {
	if m == nil {
		return nil
	}
	port, err := strconv.Atoi(m.Port)
	if err != nil {
		port = 587
	}
	auth := smtp.PlainAuth("", m.User, m.Password, m.Host)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.From, m.To, subject, body))
	addr := fmt.Sprintf("%s:%d", m.Host, port)
	return smtp.SendMail(addr, auth, m.From, []string{m.To}, msg)
}
