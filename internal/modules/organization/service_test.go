package organization

import (
	"testing"
)

func TestCreateInput_Fields(t *testing.T) {
	input := CreateInput{
		Name:        "Test Org",
		Description: "A test organization",
	}

	if input.Name != "Test Org" {
		t.Errorf("Name = %q, want %q", input.Name, "Test Org")
	}
	if input.Description != "A test organization" {
		t.Errorf("Description = %q, want %q", input.Description, "A test organization")
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService should not return nil")
	}
	if svc.repo != nil {
		t.Error("repo should be nil when passed nil")
	}
}
