package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseParams_Defaults(t *testing.T) {
	params, err := ParseParams("", "")
	require.NoError(t, err)
	assert.Equal(t, DefaultLimit, params.Limit)
	assert.Equal(t, "", params.CursorID())
	assert.Equal(t, DefaultLimit+1, params.FetchLimit())
}

func TestParseParams_CustomLimit(t *testing.T) {
	params, err := ParseParams("", "50")
	require.NoError(t, err)
	assert.Equal(t, 50, params.Limit)
	assert.Equal(t, 51, params.FetchLimit())
}

func TestParseParams_InvalidLimit(t *testing.T) {
	_, err := ParseParams("", "0")
	require.Error(t, err)

	_, err = ParseParams("", "101")
	require.Error(t, err)

	_, err = ParseParams("", "abc")
	require.Error(t, err)
}

func TestParseParams_ValidCursor(t *testing.T) {
	encoded := EncodeCursor("019abcde-1234-7def-8901-234567890abc")
	params, err := ParseParams(encoded, "10")
	require.NoError(t, err)
	assert.Equal(t, "019abcde-1234-7def-8901-234567890abc", params.CursorID())
	assert.Equal(t, 10, params.Limit)
}

func TestParseParams_InvalidCursor(t *testing.T) {
	_, err := ParseParams("not-valid-base64!!!", "10")
	require.Error(t, err)
}

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	id := "0192f5e0-1234-7abc-def0-1234567890ab"
	encoded := EncodeCursor(id)
	decoded, err := DecodeCursor(encoded)
	require.NoError(t, err)
	assert.Equal(t, id, decoded)
}
