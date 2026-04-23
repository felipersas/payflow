package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Email    string `validate:"required,email"`
	Currency string `validate:"required,len=3"`
	Amount   int64  `validate:"required,gt=0"`
	Name     string `validate:"required,min=2,max=100"`
}

func TestValidate_Valid(t *testing.T) {
	s := testStruct{
		Email:    "user@example.com",
		Currency: "BRL",
		Amount:   1000,
		Name:     "John",
	}
	assert.NoError(t, Validate(&s))
}

func TestValidate_Required(t *testing.T) {
	s := testStruct{}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields, "email")
	assert.Contains(t, valErr.Fields, "currency")
	assert.Contains(t, valErr.Fields, "amount")
	assert.Contains(t, valErr.Fields, "name")
}

func TestValidate_Email(t *testing.T) {
	s := testStruct{Email: "not-an-email", Currency: "BRL", Amount: 100, Name: "Jo"}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields["email"], "valid email")
}

func TestValidate_Len(t *testing.T) {
	s := testStruct{Email: "a@b.com", Currency: "BR", Amount: 100, Name: "Jo"}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields["currency"], "exactly 3 characters")
}

func TestValidate_Gt(t *testing.T) {
	type gtTest struct {
		Amount int64 `validate:"gt=0"`
	}
	s := gtTest{Amount: 0}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields["amount"], "greater than")
}

func TestValidate_Min(t *testing.T) {
	s := testStruct{Email: "a@b.com", Currency: "BRL", Amount: 100, Name: "J"}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields["name"], "at least 2 characters")
}

func TestValidate_Max(t *testing.T) {
	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}
	s := testStruct{Email: "a@b.com", Currency: "BRL", Amount: 100, Name: string(longName)}
	err := Validate(&s)
	require.Error(t, err)

	valErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Contains(t, valErr.Fields["name"], "at most 100 characters")
}

func TestValidate_NilInput(t *testing.T) {
	err := Validate(nil)
	require.Error(t, err)
}

func TestValidate_NonStruct(t *testing.T) {
	err := Validate("hello")
	require.Error(t, err)
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Email", "email"},
		{"FromAccountID", "from_account_i_d"},
		{"Currency", "currency"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, toSnakeCase(tt.input))
	}
}
