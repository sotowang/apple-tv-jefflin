package media

import "testing"

func TestClassifyRights(t *testing.T) {
	cases := []struct {
		name, license, rights string
		want                  RightsStatus
	}{
		{"cc url", "https://creativecommons.org/licenses/by/4.0/", "", RightsVerified},
		{"public domain", "", "Public Domain", RightsVerified},
		{"unknown collection", "", "collection: opensource_movies", RightsUnknown},
		{"restricted overrides license", "https://creativecommons.org/licenses/by/4.0/", "All rights reserved", RightsRestricted},
		{"unrecognized license", "https://example.org/license", "", RightsUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyRights(tc.license, tc.rights); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
