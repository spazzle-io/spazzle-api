package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGrpcLogger(t *testing.T) {
	mockInfo := &grpc.UnaryServerInfo{
		FullMethod: "SomeService/SomeMethod",
	}

	mockHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "MockResponse", nil
	}

	resp, err := GrpcLogger(context.Background(), "mockRequest", mockInfo, mockHandler)

	assert.NoError(t, err)
	assert.Equal(t, "MockResponse", resp)
}

func TestHTTPLogger(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte("test_data"))
		require.NoError(t, err)
	})

	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	rec := httptest.NewRecorder()

	httpLogger := HTTPLogger(mockHandler)
	httpLogger.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "test_data")
}
