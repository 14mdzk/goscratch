package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion_Default(t *testing.T) {
	assert.NotEmpty(t, Version)
}
