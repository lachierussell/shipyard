package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestPotName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example-com"},
		{"my-app", "my-app"},
		{"a.b.c", "a-b-c"},
		{"nodots", "nodots"},
		{"sub.domain.example.com", "sub-domain-example-com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := potName(tt.input)
			if got != tt.want {
				t.Errorf("potName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestServiceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example_com"},
		{"my-app", "my_app"},
		{"a.b-c", "a_b_c"},
		{"nodots", "nodots"},
		{"sub.domain-name.com", "sub_domain_name_com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := serviceName(tt.input)
			if got != tt.want {
				t.Errorf("serviceName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRcdTemplateRender(t *testing.T) {
	data := rcdData{
		ServiceName: "example_com",
		PotName:     "example-com",
		BinaryPath:  "/usr/local/bin/example.com",
		ListenPort:  8080,
	}

	var buf bytes.Buffer
	if err := rcdTmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template execute: %v", err)
	}

	output := buf.String()

	checks := []string{
		"example_com",
		"example-com",
		"/usr/local/bin/example.com",
		"8080",
		"MANAGED BY SHIPYARD",
	}

	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("template output missing %q", want)
		}
	}
}
