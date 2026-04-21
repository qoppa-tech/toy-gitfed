package errors

import (
	stdErrors "errors"
	"testing"
)

func TestNewAndIsCode(t *testing.T) {
	err := New(CodeInvalidArgument, "invalid did")

	if !IsCode(err, CodeInvalidArgument) {
		t.Fatalf("expected code %q", CodeInvalidArgument)
	}
	if IsCode(err, CodeInternal) {
		t.Fatalf("did not expect code %q", CodeInternal)
	}
}

func TestWrapPreservesCause(t *testing.T) {
	cause := stdErrors.New("database unavailable")
	err := Wrap(cause, CodeInternal, "create peer")

	if !stdErrors.Is(err, cause) {
		t.Fatalf("wrapped error should preserve cause")
	}
	if !IsCode(err, CodeInternal) {
		t.Fatalf("expected code %q", CodeInternal)
	}
}

func TestCodeOfUnknownError(t *testing.T) {
	if _, ok := CodeOf(stdErrors.New("plain")); ok {
		t.Fatalf("plain errors should not expose app code")
	}
}

func TestNotImplementedHelper(t *testing.T) {
	err := NotImplemented("peers endpoint")
	if !IsCode(err, CodeNotImplemented) {
		t.Fatalf("expected not_implemented code")
	}
}
