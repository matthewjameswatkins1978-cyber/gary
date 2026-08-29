package main

import (
	"strings"
	"testing"
)

func TestYardmasterVertexEndpointUsesGlobalPublisherModel(t *testing.T) {
	got := yardmasterVertexEndpoint("gary-agent-yard", "gemini-3.7-flash")
	want := "https://aiplatform.googleapis.com/v1/projects/gary-agent-yard/locations/global/publishers/google/models/gemini-3.7-flash:generateContent"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestYardmasterVertexEndpointDoesNotUseDeveloperAPI(t *testing.T) {
	got := yardmasterVertexEndpoint("p", "m")
	if strings.Contains(got, "generativelanguage.googleapis.com") {
		t.Fatalf("Yardmaster must use Vertex control-plane auth, got %q", got)
	}
	if !strings.Contains(got, "/locations/global/") {
		t.Fatalf("Yardmaster should use global Vertex endpoint, got %q", got)
	}
}
