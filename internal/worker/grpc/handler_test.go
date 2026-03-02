package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"weather/internal/worker/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "deadline exceeded wrapped",
			err:  fmt.Errorf("wrap: %w", context.DeadlineExceeded),
			want: codes.DeadlineExceeded,
		},
		{
			name: "canceled wrapped",
			err:  fmt.Errorf("wrap: %w", context.Canceled),
			want: codes.Canceled,
		},
		{
			name: "invalid argument",
			err:  fmt.Errorf("wrap: %w", domain.ErrInvalidDays),
			want: codes.InvalidArgument,
		},
		{
			name: "not found",
			err:  fmt.Errorf("wrap: %w", domain.ErrCityNotFound),
			want: codes.NotFound,
		},
		{
			name: "internal",
			err:  errors.New("boom"),
			want: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErr(tt.err)
			st, ok := status.FromError(got)
			if !ok {
				t.Fatalf("expected grpc status error, got: %v", got)
			}
			if st.Code() != tt.want {
				t.Fatalf("expected code %s, got %s", tt.want, st.Code())
			}
		})
	}
}
