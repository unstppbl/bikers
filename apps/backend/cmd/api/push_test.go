package main

import (
	"bikers.kz/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushKeysPrivateFileAndNoOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "push.env")
	args := []string{"--output", path, "--subject", "mailto:operator@example.com"}
	if err := runPushKeys(args); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("keys file must be readable only by its owner")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(before)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatal("invalid generated environment file")
		}
		if key == "WEB_PUSH_SUBJECT" {
			if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
				t.Fatal("contact URI must be literal in Compose")
			}
			value = strings.ReplaceAll(value[1:len(value)-1], "\\'", "'")
		}
		values[key] = value
	}
	if err = config.ValidatePush(values["WEB_PUSH_PUBLIC_KEY"], values["WEB_PUSH_PRIVATE_KEY"], values["WEB_PUSH_SUBJECT"]); err != nil {
		t.Fatal("generated configuration is invalid")
	}
	if err = runPushKeys(args); err == nil {
		t.Fatal("must not replace existing keys")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("existing keys changed")
	}
}

func TestPushKeysInvalidInputCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "push.env")
	for _, args := range [][]string{nil, {"--output", path}, {"--output", path, "--subject", "http://example.com"}, {"--output", path, "--subject", "mailto:a@example.com\nEVIL=1"}} {
		if err := runPushKeys(args); err == nil {
			t.Fatal("invalid input accepted")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("invalid input created a file")
		}
	}
}

func TestPushKeysSubjectIsLiteralComposeEnvironmentValue(t *testing.T) {
	for _, contact := range []string{"https://example.com/contact?team=$TEAM", "mailto:o'connor@example.com"} {
		path := filepath.Join(t.TempDir(), "push.env")
		if err := runPushKeys([]string{"--output", path, "--subject", contact}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := "WEB_PUSH_SUBJECT='" + strings.ReplaceAll(contact, "'", "\\'") + "'\n"
		if !strings.HasSuffix(string(data), expected) {
			t.Fatal("contact URI was not serialized as a literal Compose value")
		}
	}
}
