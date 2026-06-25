package authidentity

import "strings"

const (
	MaxClassicUsernameLen = 16
	MaxEmailLen           = 255
)

func NormalizeLoginIdentity(identity string) string {
	return strings.TrimSpace(identity)
}

func IsValidLoginIdentity(identity string) bool {
	normalized := NormalizeLoginIdentity(identity)
	if identity != normalized {
		return false
	}
	return IsValidClassicUsername(normalized)
}

func IsValidClassicUsername(username string) bool {
	if len(username) == 0 || len(username) > MaxClassicUsernameLen {
		return false
	}
	for i := 0; i < len(username); i++ {
		c := username[i]
		if isASCIILetter(c) || isASCIIDigit(c) || c == '_' || c == '-' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func IsValidEmail(email string) bool {
	normalized := NormalizeLoginIdentity(email)
	if email != normalized {
		return false
	}
	email = normalized
	if len(email) == 0 || len(email) > MaxEmailLen || !isPrintableASCII(email) {
		return false
	}
	if strings.Count(email, "@") != 1 {
		return false
	}

	parts := strings.Split(email, "@")
	local, domain := parts[0], parts[1]
	if !isValidEmailLocalPart(local) || !isValidEmailDomain(domain) {
		return false
	}
	return true
}

func isValidEmailLocalPart(local string) bool {
	if len(local) == 0 || len(local) > 64 {
		return false
	}
	if local[0] == '.' || local[len(local)-1] == '.' || strings.Contains(local, "..") {
		return false
	}
	for i := 0; i < len(local); i++ {
		c := local[i]
		if isASCIILetter(c) || isASCIIDigit(c) {
			continue
		}
		switch c {
		case '.', '_', '%', '+', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func isValidEmailDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || !strings.Contains(domain, ".") {
		return false
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !isASCIILetter(c) && !isASCIIDigit(c) && c != '-' {
				return false
			}
		}
	}
	return true
}

func isPrintableASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7E {
			return false
		}
	}
	return true
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
