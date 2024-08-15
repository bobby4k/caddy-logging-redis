package loggingredis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisStorage_ComressUncompress(t *testing.T) {

	rs := New()
	originalValue := []byte("Q2FkZHkgUmVkaXMgU3RvcmFnZQ==")

	compressedValue, err := rs.compress(originalValue)
	assert.NoError(t, err)

	uncompressedValue, err := rs.uncompress(compressedValue)
	assert.NoError(t, err)
	assert.Equal(t, originalValue, uncompressedValue)
}
