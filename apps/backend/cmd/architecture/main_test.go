package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module fixture.example/backend\n\ngo 1.27.3\n"
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAllowedDependenciesAndExternalOwnerTests(t *testing.T) {
	root := fixture(t, map[string]string{
		"internal/models/value.go":               "package models\nimport _ \"strings\"\n",
		"internal/config/settings.go":            "package config\nimport _ \"os\"\n",
		"internal/database/contracts.go":         "package database\nimport _ \"fixture.example/backend/internal/models\"\n",
		"internal/database/postgres/store.go":    "package postgres\nimport ( _ \"fixture.example/backend/internal/database\"; _ \"fixture.example/backend/internal/models\" )\n",
		"internal/managers/accounts/account.go":  "package accounts\nimport ( _ \"fixture.example/backend/internal/database\"; _ \"fixture.example/backend/internal/models\"; _ \"fixture.example/backend/internal/managers/rides\" )\n",
		"internal/clients/telegram/gateway.go":   "package telegram\nimport _ \"fixture.example/backend/internal/models\"\n",
		"internal/resources/http/routes.go":      "package httpapi\nimport ( _ \"fixture.example/backend/internal/managers/accounts\"; _ \"fixture.example/backend/internal/models\" )\n",
		"internal/workers/poll.go":               "package workers\nimport ( _ \"fixture.example/backend/internal/managers/rooms\"; _ \"fixture.example/backend/internal/models\" )\n",
		"internal/app/wiring_test.go":            "package app_test\nimport ( _ \"fixture.example/backend/internal/app\"; _ \"fixture.example/backend/internal/config\"; _ \"fixture.example/backend/internal/database/postgres\"; _ \"fixture.example/backend/internal/clients/telegram\"; _ \"fixture.example/backend/internal/resources/http\" )\n",
		"internal/models/value_test.go":          "package models_test\nimport _ \"fixture.example/backend/internal/models\"\n",
		"internal/resources/http/routes_test.go": "package httpapi_test\nimport _ \"fixture.example/backend/internal/resources/http\"\n",
	})
	violations, err := check(root)
	if err != nil || len(violations) != 0 {
		t.Fatalf("allowed imports rejected: %v, %v", violations, err)
	}
}

func TestForbiddenDependenciesIncludingTestsAndBuildTags(t *testing.T) {
	cases := []struct{ name, path, source string }{
		{"model_to_manager", "internal/models/value.go", "package models\nimport _ \"fixture.example/backend/internal/managers/rides\"\n"},
		{"config_to_model", "internal/config/settings.go", "package config\nimport _ \"fixture.example/backend/internal/models\"\n"},
		{"contract_to_implementation", "internal/database/contracts.go", "package database\nimport _ \"fixture.example/backend/internal/database/postgres\"\n"},
		{"repository_to_manager", "internal/database/postgres/store.go", "package postgres\nimport _ \"fixture.example/backend/internal/managers/rides\"\n"},
		{"manager_to_repository", "internal/managers/rides/ride.go", "package rides\nimport _ \"fixture.example/backend/internal/database/postgres\"\n"},
		{"manager_to_client", "internal/managers/accounts/account.go", "package accounts\nimport _ \"fixture.example/backend/internal/clients/telegram\"\n"},
		{"client_to_config", "internal/clients/telegram/gateway.go", "package telegram\nimport _ \"fixture.example/backend/internal/config\"\n"},
		{"resource_to_database", "internal/resources/http/routes.go", "package httpapi\nimport _ \"fixture.example/backend/internal/database\"\n"},
		{"worker_to_nonqueue_client", "internal/workers/poll.go", "package workers\nimport _ \"fixture.example/backend/internal/clients/telegram\"\n"},
		{"same_package_test", "internal/resources/http/routes_test.go", "package httpapi\nimport _ \"fixture.example/backend/internal/app\"\n"},
		{"external_test_not_exempt", "internal/models/value_test.go", "package models_test\nimport _ \"fixture.example/backend/internal/config\"\n"},
		{"excluded_build_tag", "internal/managers/rides/tagged.go", "//go:build architecture_never_enabled\n\npackage rides\nimport _ \"fixture.example/backend/internal/config\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, map[string]string{tc.path: tc.source})
			violations, err := check(root)
			if err != nil || len(violations) != 1 || !strings.Contains(violations[0], "must not import") {
				t.Fatalf("expected one import violation, got %v, %v", violations, err)
			}
		})
	}
}

func TestUnknownAndNestedPackagePaths(t *testing.T) {
	for _, path := range []string{"internal/helpers/helper.go", "internal/managers/rides/private/helper.go", "internal/clients/Telegram/gateway.go", "internal/models/extra/value.go"} {
		t.Run(path, func(t *testing.T) {
			root := fixture(t, map[string]string{path: "package fixture\n"})
			violations, err := check(root)
			if err != nil || len(violations) != 1 || !strings.Contains(violations[0], "outside the approved layout") {
				t.Fatalf("expected layout violation, got %v, %v", violations, err)
			}
		})
	}
}

func TestUnknownImportAndDiagnosticLocation(t *testing.T) {
	root := fixture(t, map[string]string{"internal/app/wiring.go": "package app\nimport _ \"fixture.example/backend/internal/unapproved/helper\"\n"})
	violations, err := check(root)
	if err != nil || len(violations) != 1 || !strings.Contains(violations[0], "internal/app/wiring.go:2:") {
		t.Fatalf("expected located unknown import violation, got %v, %v", violations, err)
	}
}

func TestCommandExitStatuses(t *testing.T) {
	var out, errOut bytes.Buffer
	root := fixture(t, map[string]string{"internal/models/value.go": "package models\n"})
	if got := run([]string{"-root", root}, &out, &errOut); got != 0 || !strings.Contains(out.String(), "passed") {
		t.Fatalf("expected successful command, exit=%d err=%s", got, errOut.String())
	}
	root = fixture(t, map[string]string{"internal/models/value.go": "package models\nimport _ \"fixture.example/backend/internal/app\"\n"})
	errOut.Reset()
	if got := run([]string{"-root", root}, &out, &errOut); got != 1 || !strings.Contains(errOut.String(), "must not import") {
		t.Fatalf("expected violation exit, exit=%d err=%s", got, errOut.String())
	}
	if got := run([]string{"-root", filepath.Join(root, "missing")}, &out, &errOut); got != 2 {
		t.Fatalf("expected configuration error exit, got %d", got)
	}
}

func TestRepositoryArchitecture(t *testing.T) {
	violations, err := check(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("internal package boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}
