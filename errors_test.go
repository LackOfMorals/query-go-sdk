package query

import (
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "404 error",
			err:  &Error{StatusCode: 404, Message: "not found"},
			want: true,
		},
		{
			name: "wrapped 404 error",
			err:  fmt.Errorf("operation failed: %w", &Error{StatusCode: 404, Message: "not found"}),
			want: true,
		},
		{
			name: "401 error",
			err:  &Error{StatusCode: 401, Message: "unauthorized"},
			want: false,
		},
		{
			name: "500 error",
			err:  &Error{StatusCode: 500, Message: "internal server error"},
			want: false,
		},
		{
			name: "non-API error",
			err:  fmt.Errorf("connection refused"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
