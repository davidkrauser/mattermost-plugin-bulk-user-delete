package main

import (
	"testing"
)

func Test_validateCommand(t *testing.T) {
	tests := []struct {
		command   string
		expectErr bool
	}{{
		command:   "",
		expectErr: true,
	}, {
		command:   "foo",
		expectErr: true,
	}, {
		command:   "foo bar",
		expectErr: true,
	}, {
		command:   "foo bar bazz",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete bar",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete bar bazz",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete dry-run",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete live",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete dry-run bazz",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete live bazz",
		expectErr: true,
	}, {
		command:   "/bulk-user-delete dry-run inactive",
		expectErr: false,
	}, {
		command:   "/bulk-user-delete live all",
		expectErr: false,
	}}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			err := validateCommand(test.command)
			if test.expectErr && err == nil {
				t.Errorf("did not get expected error")
				return
			}
			if !test.expectErr && err != nil {
				t.Errorf("got unexpected error: %s", err.Error())
				return
			}
		})
	}
}
