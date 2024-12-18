package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func Test_emailMatches(t *testing.T) {
	tests := []struct {
		description       string
		userEmail         string
		targetSuffixes    []string
		targetExactEmails []string
		expected          bool
	}{{
		description:       "empty target list should not delete",
		targetSuffixes:    []string{},
		targetExactEmails: []string{},
		userEmail:         "admin@test.com",
	}, {
		description:    "non-matching suffixes should not delete",
		targetSuffixes: []string{"@example.com"},
		userEmail:      "admin@test.com",
	}, {
		description:    "matching suffixes should delete",
		targetSuffixes: []string{"@example.com", "@test.com"},
		userEmail:      "admin@test.com",
		expected:       true,
	}, {
		description:       "non-matching exact emails should not delete",
		targetExactEmails: []string{"admin1@test.com"},
		userEmail:         "admin2@test.com",
	}, {
		description:       "matching exact emails should delete",
		targetExactEmails: []string{"admin1@test.com", "admin2@test.com"},
		userEmail:         "admin2@test.com",
		expected:          true,
	}, {
		description:       "non-matching targets should not delete",
		targetSuffixes:    []string{"@example.com"},
		targetExactEmails: []string{"admin1@test.com"},
		userEmail:         "admin2@test.com",
	}, {
		description:       "matching targets should delete, suffix matches",
		targetSuffixes:    []string{"@example.com", "@test.com"},
		targetExactEmails: []string{"admin1@test.com"},
		userEmail:         "admin2@test.com",
		expected:          true,
	}, {
		description:       "matching targets should delete, exact matches",
		targetSuffixes:    []string{"@example.com"},
		targetExactEmails: []string{"admin1@test.com", "admin2@test.com"},
		userEmail:         "admin2@test.com",
		expected:          true,
	}}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			user := model.User{Email: test.userEmail}
			got := emailMatches(&user, test.targetSuffixes, test.targetExactEmails)
			if got != test.expected {
				t.Errorf("expected: '%t', got: '%t'", test.expected, got)
			}
		})
	}
}
