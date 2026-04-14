package eleconf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ttab/eleconf"
)

func writeHCL(t *testing.T, dir, name, content string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseDocumentType(t *testing.T) {
	tests := []struct {
		input       string
		wantBase    string
		wantVariant string
	}{
		{"core/article", "core/article", ""},
		{"core/article#timeless", "core/article", "timeless"},
		{"tt/print#special", "tt/print", "special"},
	}

	for _, tt := range tests {
		base, variant := eleconf.ParseDocumentType(tt.input)
		if base != tt.wantBase || variant != tt.wantVariant {
			t.Errorf("ParseDocumentType(%q) = (%q, %q), want (%q, %q)",
				tt.input, base, variant, tt.wantBase, tt.wantVariant)
		}
	}
}

func TestReadConfigFromDirectory_VariantOnVariant(t *testing.T) {
	dir := t.TempDir()
	writeHCL(t, dir, "test.hcl", `
document "core/article" {
  variants = ["timeless"]
  statuses = ["draft"]
}
document "core/article#timeless" {
  variants = ["bad"]
  statuses = ["draft"]
}
`)

	_, err := eleconf.ReadConfigFromDirectory(dir)
	if err == nil || !strings.Contains(err.Error(), "must not define variants") {
		t.Fatalf("expected variant-on-variant error, got: %v", err)
	}
}

func TestReadConfigFromDirectory_UndeclaredVariant(t *testing.T) {
	dir := t.TempDir()
	writeHCL(t, dir, "test.hcl", `
document "core/article" {
  statuses = ["draft"]
}
document "core/article#timeless" {
  statuses = ["draft"]
}
`)

	_, err := eleconf.ReadConfigFromDirectory(dir)
	if err == nil || !strings.Contains(err.Error(), "has not been declared") {
		t.Fatalf("expected undeclared variant error, got: %v", err)
	}
}

func TestReadConfigFromDirectory_MissingBaseType(t *testing.T) {
	dir := t.TempDir()
	writeHCL(t, dir, "test.hcl", `
document "core/article#timeless" {
  statuses = ["draft"]
}
`)

	_, err := eleconf.ReadConfigFromDirectory(dir)
	if err == nil || !strings.Contains(err.Error(), "base type") {
		t.Fatalf("expected missing base type error, got: %v", err)
	}
}

func TestReadConfigFromDirectory_ValidVariantConfig(t *testing.T) {
	dir := t.TempDir()
	writeHCL(t, dir, "test.hcl", `
document "core/article" {
  variants = ["timeless"]
  statuses = ["draft", "done", "usable"]
  workflow = {
    step_zero  = "draft"
    checkpoint = "usable"
    negative_checkpoint = "unpublished"
    steps = ["draft", "done"]
  }
}
document "core/article#timeless" {
  statuses = ["draft", "done"]
  workflow = {
    step_zero  = "draft"
    checkpoint = "done"
    negative_checkpoint = "cancelled"
    steps = ["draft", "done"]
  }
}
`)

	conf, err := eleconf.ReadConfigFromDirectory(dir)
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	if len(conf.Documents) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(conf.Documents))
	}
}
