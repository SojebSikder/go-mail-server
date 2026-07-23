package config

import "os"

func GetAllowedSenderDomain() string {
	domain := os.Getenv("ALLOWED_SENDER_DOMAIN")
	if domain == "" {
		return "jabokivabe.com" // default fallback
	}
	return domain
}
