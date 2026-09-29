package media

import "strings"

type RightsStatus string

const (
	RightsVerified   RightsStatus = "verified"
	RightsUnknown    RightsStatus = "unknown"
	RightsRestricted RightsStatus = "restricted"
)

// ClassifyRights is a metadata hint, not a legal determination. A collection name alone
// cannot establish permission to use an item.
func ClassifyRights(licenseURL, rights string) RightsStatus {
	text := strings.ToLower(strings.TrimSpace(rights))
	for _, marker := range []string{"all rights reserved", "restricted", "permission required", "copyrighted"} {
		if strings.Contains(text, marker) {
			return RightsRestricted
		}
	}
	license := strings.ToLower(strings.TrimSpace(licenseURL))
	if strings.Contains(text, "public domain") || strings.Contains(text, "creative commons") || strings.Contains(text, "cc-by") ||
		strings.HasPrefix(license, "https://creativecommons.org/licenses/") || strings.HasPrefix(license, "http://creativecommons.org/licenses/") ||
		strings.HasPrefix(license, "https://creativecommons.org/publicdomain/") || strings.HasPrefix(license, "http://creativecommons.org/publicdomain/") {
		return RightsVerified
	}
	return RightsUnknown
}
