package entity_test

import (
	"testing"

	"server-template/internal/domain/entity"
)

func TestNormalizeEmail(t *testing.T) {
	if got := entity.NormalizeEmail("  Foo@Example.COM \n"); got != "foo@example.com" {
		t.Fatalf("got %q", got)
	}
}
