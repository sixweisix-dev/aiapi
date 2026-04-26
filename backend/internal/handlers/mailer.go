package handlers

import (
        "fmt"
        "net/smtp"
        "strconv"
)

type MailConfig struct {
        Host     string
        Port     string
        User     string
        Password string
        From     string
}

func SendResetEmail(cfg MailConfig, toEmail, resetURL string) error {
        port, err := strconv.Atoi(cfg.Port)
        if err != nil {
                port = 587
        }

        auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)

        subject := "TransitAI 密码重置"
        body := fmt.Sprintf(`您好，

您申请了密码重置。请点击以下链接重置密码（30分钟内有效）：

%s

如果您没有申请密码重置，请忽略此邮件。

TransitAI 团队`, resetURL)

        msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
                cfg.From, toEmail, subject, body)

        addr := fmt.Sprintf("%s:%d", cfg.Host, port)
        return smtp.SendMail(addr, auth, cfg.From, []string{toEmail}, []byte(msg))
}
