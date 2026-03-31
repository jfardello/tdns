package log

import (
	"testing"

	logrus "github.com/sirupsen/logrus"
)

func TestConfigureVerboseOverridesConfigLevel(t *testing.T) {
	level := Configure("error", true)

	if level != logrus.DebugLevel {
		t.Fatalf("expected debug level, got %s", level)
	}
	if !IsDebugEnabled() {
		t.Fatal("expected debug mode to be enabled")
	}
}

func TestConfigureUsesConfigLevelWhenVerboseDisabled(t *testing.T) {
	level := Configure("error", false)

	if level != logrus.ErrorLevel {
		t.Fatalf("expected error level, got %s", level)
	}
	if IsDebugEnabled() {
		t.Fatal("expected debug mode to be disabled")
	}
}

func TestConfigureFallsBackToInfoOnInvalidLevel(t *testing.T) {
	level := Configure("definitely-not-a-level", false)

	if level != logrus.InfoLevel {
		t.Fatalf("expected info level fallback, got %s", level)
	}
}
