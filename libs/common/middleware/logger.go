package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GrpcLogger(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	startTime := time.Now()

	result, err := handler(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("grpc call failed")
	}

	duration := time.Since(startTime)

	logger := log.Info()

	statusCode := codes.Unknown
	st, ok := status.FromError(err)
	if ok {
		statusCode = st.Code()
	}

	ip, ok := ctx.Value(ClientIP).(string)
	if !ok {
		ip = "unknown"
	}

	if authenticatedService, ok := ctx.Value(AuthenticatedService).(string); ok {
		logger = logger.Str("authenticated_service", authenticatedService)
	}

	logger.Str("protocol", "grpc").
		Str("method", info.FullMethod).
		Int("status_code", int(statusCode)).
		Str("status_text", statusCode.String()).
		Str("client_ip", ip).
		Dur("duration", duration).
		Msg("processed gRPC request")

	return result, err
}

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       []byte
}

func (rec *ResponseRecorder) WriteHeader(statusCode int) {
	if rec.StatusCode == 0 {
		rec.StatusCode = statusCode
		rec.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rec *ResponseRecorder) Write(body []byte) (int, error) {
	rec.Body = append(rec.Body, body...)
	return rec.ResponseWriter.Write(body)
}

func (rec *ResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rec.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}

func (rec *ResponseRecorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func HTTPLogger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		rec := &ResponseRecorder{
			ResponseWriter: res,
			StatusCode:     0,
		}
		handler.ServeHTTP(rec, req)
		if rec.StatusCode == 0 {
			rec.StatusCode = http.StatusOK
		}
		duration := time.Since(startTime)

		logger := log.Info()

		if rec.StatusCode >= 400 && rec.StatusCode < 600 {
			logger = log.Error().Bytes("body", rec.Body)
		}

		ip, ok := req.Context().Value(ClientIP).(string)
		if !ok {
			ip = "unknown"
		}

		if authenticatedService, ok := req.Context().Value(AuthenticatedService).(string); ok {
			logger = logger.Str("authenticated_service", authenticatedService)
		}

		logger.Str("protocol", "http").
			Str("method", req.Method).
			Str("path", req.RequestURI).
			Int("status_code", rec.StatusCode).
			Str("status_text", http.StatusText(rec.StatusCode)).
			Str("client_ip", ip).
			Dur("duration", duration).
			Msg("processed HTTP request")
	})
}
