package callback

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

// EmailAlerter sends email alerts for budget threshold breaches.
// Not a BatchLogger — sends individual alerts on budget events.
type EmailAlerter struct {
	host     string
	port     int
	from     string
	to       []string
	username string
	password string
	useTLS   bool
}

// NewEmailAlerter creates an email alerter.
func NewEmailAlerter(host string, port int, from string, to []string, username, password string) *EmailAlerter {
	return &EmailAlerter{
		host:     host,
		port:     port,
		from:     from,
		to:       to,
		username: username,
		password: password,
		useTLS:   port == 465,
	}
}

// LogSuccess sends an alert if the log data indicates a budget threshold breach.
func (e *EmailAlerter) LogSuccess(data LogData) {
	// Email alerts are triggered by budget events, not regular log data.
	// The proxy budget middleware should call SendAlert directly.
}

// LogFailure is a no-op for email alerting.
func (e *EmailAlerter) LogFailure(data LogData) {}

// SendAlert sends a budget alert email.
func (e *EmailAlerter) SendAlert(subject, body string) error {
	htmlBody, err := renderAlertEmail(subject, body)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		e.from, strings.Join(e.to, ","), subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	if e.useTLS {
		return e.sendTLS(addr, msg)
	}
	return e.sendSTARTTLS(addr, msg)
}

func (e *EmailAlerter) sendSTARTTLS(addr, msg string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	return smtp.SendMail(addr, auth, e.from, e.to, []byte(msg))
}

func (e *EmailAlerter) sendTLS(addr, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.host})
	if err != nil {
		return fmt.Errorf("email tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		return fmt.Errorf("email smtp client: %w", err)
	}
	defer client.Close()

	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("email auth: %w", err)
	}

	if err = client.Mail(e.from); err != nil {
		return err
	}
	for _, to := range e.to {
		if err = client.Rcpt(to); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return w.Close()
}

var alertTemplate = template.Must(template.New("alert").Parse(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; padding: 20px;">
<h2 style="color: #d32f2f;">{{.Subject}}</h2>
<p>{{.Body}}</p>
<hr>
<p style="color: #666; font-size: 12px;">Sent by TianjiLLM Proxy</p>
</body>
</html>
`))

func renderAlertEmail(subject, body string) (string, error) {
	var buf strings.Builder
	err := alertTemplate.Execute(&buf, struct {
		Subject string
		Body    string
	}{subject, body})
	return buf.String(), err
}
