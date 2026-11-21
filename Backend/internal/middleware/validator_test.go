package middleware

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

type registerShape struct {
	Email    string `validate:"required"`
	Password string `validate:"required,min=8"`
	Name     string
}

type loginShape struct {
	Email    string `validate:"required"`
	Password string `validate:"required"`
}

func messageOf(t *testing.T, err error) (int, string) {
	t.Helper()
	var he *echo.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("err = %v, want HTTPError", err)
	}
	msg, ok := he.Message.(APIError)
	if !ok {
		t.Fatalf("message = %#v", he.Message)
	}
	if msg.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %s", msg.Code)
	}
	return he.Code, msg.Msg
}

func TestCustomValidator_KeepsAcceptedRegistration(t *testing.T) {
	err := NewCustomValidator().Validate(registerShape{
		Email:    "not-an-email",
		Password: "12345678",
	})
	if err != nil {
		t.Fatalf("名前なし・形式を問わないメールは今も受け付ける: %v", err)
	}
}

func TestCustomValidator_RejectsShortPassword(t *testing.T) {
	err := NewCustomValidator().Validate(registerShape{Email: "a@b.c", Password: "short"})
	code, msg := messageOf(t, err)
	if code != http.StatusBadRequest || msg != "password must be at least 8 characters" {
		t.Fatalf("got %d %s", code, msg)
	}
}

func TestCustomValidator_RejectsEmptyLogin(t *testing.T) {
	err := NewCustomValidator().Validate(loginShape{})
	code, msg := messageOf(t, err)
	if code != http.StatusBadRequest || msg != "email and password are required" {
		t.Fatalf("got %d %s", code, msg)
	}
}
