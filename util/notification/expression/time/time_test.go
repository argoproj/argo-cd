package time

import (
	"testing"
	"time"

	"github.com/antonmedv/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTimeExprs(t *testing.T) {
	funcs := []string{
		"Parse",
		"Now",
		"Nanosecond",
		"Microsecond",
		"Millisecond",
		"Second",
		"Minute",
		"Hour",
		"Layout",
		"ANSIC",
		"UnixDate",
		"RubyDate",
		"RFC822",
		"RFC822Z",
		"RFC850",
		"RFC1123",
		"RFC1123Z",
		"RFC3339",
		"RFC3339Nano",
		"Kitchen",
		"Stamp",
		"StampMilli",
		"StampMicro",
		"StampNano",
	}

	for _, fn := range funcs {
		timeExprs := NewExprs()
		_, exists := timeExprs[fn]
		assert.True(t, exists)
	}
}

func Test_NewExprs_Now(t *testing.T) {
	defer func() { now = time.Now }()
	fixedTime := time.Date(2022, 9, 26, 11, 30, 25, 0, time.UTC)
	now = func() time.Time {
		return fixedTime
	}

	vm, err := expr.Compile("time.Now().Truncate(time.Hour).Format(time.RFC3339)")
	require.NoError(t, err)

	val, err := expr.Run(vm, map[string]interface{}{"time": NewExprs()})
	require.NoError(t, err)

	assert.Equal(t, "2022-09-26T11:00:00Z", val)
}
