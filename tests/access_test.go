package tests

import (
	"testing"

	sitemanager "github.com/veltylabs/site_manager"
)

func TestPendingRequestsReturnsOnlyPending(t *testing.T) {
	m, _ := setupModule(t)

	first, _, err := m.Request("a@example.com", "A", "hola")
	if err != nil {
		t.Fatalf("Request(a): %v", err)
	}
	if _, _, err := m.Request("b@example.com", "B", "hola"); err != nil {
		t.Fatalf("Request(b): %v", err)
	}

	if err := m.AcceptRequest(first.Id); err != nil {
		t.Fatalf("AcceptRequest: %v", err)
	}

	var pending []sitemanager.AccessRequest
	pending, err = m.PendingRequests()
	if err != nil {
		t.Fatalf("PendingRequests: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}
	if pending[0].Email != "b@example.com" {
		t.Fatalf("expected pending request from b@example.com, got %s", pending[0].Email)
	}
}

func TestPendingRequestsEmpty(t *testing.T) {
	m, _ := setupModule(t)

	pending, err := m.PendingRequests()
	if err != nil {
		t.Fatalf("PendingRequests: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending requests, got %d", len(pending))
	}
}

func TestRequestByIDReturnsMatch(t *testing.T) {
	m, _ := setupModule(t)

	req, _, err := m.Request("a@example.com", "A", "hola")
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	var got sitemanager.AccessRequest
	got, err = m.RequestByID(req.Id)
	if err != nil {
		t.Fatalf("RequestByID: %v", err)
	}
	if got.Id != req.Id || got.Email != "a@example.com" {
		t.Fatalf("expected request %s (a@example.com), got %s (%s)", req.Id, got.Id, got.Email)
	}
}

func TestRequestByIDNotFound(t *testing.T) {
	m, _ := setupModule(t)

	if _, err := m.RequestByID("does-not-exist"); err == nil {
		t.Fatal("expected error for nonexistent id, got nil")
	}
}
