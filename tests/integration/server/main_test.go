package server

import (
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
)

func TestMain(m *testing.M) {
	setup.RunTestsWithSingleton(m)
}
