package qortoo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorFormatting(t *testing.T) {
	tests := []struct {
		name string
		code int32
		msg  string
		want string
	}{
		{"native identifier", 100, "[E100] invalid collection name", "qortoo: [E100] invalid collection name"},
		{"plain message", 999, "callback panicked", "qortoo: [E999] callback panicked"},
		{"empty message", 206, "", "qortoo: [E206]"},
		{"identifier only", 206, "[E206]", "qortoo: [E206]"},
		{"nested error", 210, "[E210] sync failed: [E306] server internal error", "qortoo: [E210] sync failed: [E306] server internal error"},
		{"different identifier", 210, "[E306] server internal error", "qortoo: [E210] [E306] server internal error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{Code: tt.code, Msg: tt.msg}
			require.Equal(t, tt.want, err.Error())
			require.Equal(t, tt.msg, err.Msg)
			require.ErrorIs(t, err, &Error{Code: tt.code, Msg: "different detail"})
		})
	}
}
