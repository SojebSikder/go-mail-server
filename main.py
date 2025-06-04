import smtplib
from email.message import EmailMessage

msg = EmailMessage()
msg.set_content("Hello from smtp server")
msg["Subject"] = "Test Email"
msg["From"] = "sender@example.com"
msg["To"] = "test@example.com"

with smtplib.SMTP("localhost", 2525) as s:
    s.send_message(msg)
