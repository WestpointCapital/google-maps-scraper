package gmaps

import "strings"

// CsvEmailColumnCount is how many separate email columns appear in CSV export (email_1 … email_N).
const CsvEmailColumnCount = 10

// SanitizeAndDedupeEmails parses addresses with the same rules as mailto extraction, removes
// obvious template / example addresses, and de-duplicates case-insensitively (first spelling kept).
func SanitizeAndDedupeEmails(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	out := make([]string, 0, len(emails))

	for _, raw := range emails {
		e, err := getValidEmail(raw)
		if err != nil {
			continue
		}

		if isJunkOrPlaceholderEmail(e) {
			continue
		}

		key := strings.ToLower(e)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, e)
	}

	return out
}

// emailsCsvSlots returns exactly CsvEmailColumnCount cells for CSV rows (padded with empty strings).
func emailsCsvSlots(emails []string) []string {
	clean := SanitizeAndDedupeEmails(emails)
	out := make([]string, CsvEmailColumnCount)
	for i := 0; i < CsvEmailColumnCount; i++ {
		if i < len(clean) {
			out[i] = clean[i]
		}
	}

	return out
}

func isJunkOrPlaceholderEmail(email string) bool {
	lower := strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(lower, "@")
	if at <= 0 || at == len(lower)-1 {
		return true
	}

	local := lower[:at]
	host := lower[at+1:]

	// Exact-address boilerplate (often copied from tutorials / WYSIWYG defaults).
	switch lower {
	case "user@domain.com", "you@example.com", "yourname@yourdomain.com",
		"email@email.com", "name@email.com", "admin@domain.com",
		"test@test.com", "user@example.com", "username@example.com":
		return true
	}

	// RFC / reserved example namespaces.
	if host == "example.com" || host == "example.org" || host == "example.net" || host == "example.edu" {
		return true
	}
	if strings.HasPrefix(host, "example.") {
		return true
	}

	// Common test / placeholder hosts.
	switch host {
	case "test.com", "test.org", "test", "invalid", "localhost":
		return true
	}

	// Suspicious “template” local parts on generic placeholder domains.
	if (local == "user" || local == "admin" || local == "test") && host == "domain.com" {
		return true
	}

	return false
}
