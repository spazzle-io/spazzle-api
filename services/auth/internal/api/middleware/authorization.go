package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"google.golang.org/grpc/metadata"
)

const (
	authorizationHeader = "authorization"
	authorizationBearer = "bearer"
)

func Authorize(
	ctx context.Context,
	userId uuid.UUID,
	tokenMaker token.Maker,
	tokenType token.Type,
	authorizedRoles []token.Role,
) (*token.Payload, error) {
	mtdt, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("could not get metadata from incoming context")
	}

	authValues := mtdt.Get(authorizationHeader)
	if len(authValues) == 0 {
		return nil, errors.New("missing authorization header")
	}

	authHeader := authValues[0]
	fields := strings.Fields(authHeader)
	if len(fields) != 2 {
		return nil, errors.New("invalid authorization header format")
	}

	authType := fields[0]
	if !strings.EqualFold(authorizationBearer, authType) {
		return nil, fmt.Errorf("unsupported authorization type: %s", authType)
	}

	tk := fields[1]
	payload, err := tokenMaker.VerifyToken(tk)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization token: %s", err)
	}

	if payload.TokenType != tokenType {
		return nil, fmt.Errorf("token type mismatch: expected '%s', got '%s'", tokenType, payload.TokenType)
	}

	if !isAuthorizedRole(payload.Role, authorizedRoles) {
		return nil, fmt.Errorf("unauthorized role: role '%s' is not allowed", payload.Role)
	}

	if payload.Role != token.Admin && userId != payload.UserId {
		return nil, fmt.Errorf("unauthorized access: user ID '%s' does not match token owner '%s'", userId, payload.UserId)
	}

	return payload, nil
}

func isAuthorizedRole(role token.Role, allowedRoles []token.Role) bool {
	for _, r := range allowedRoles {
		if r == role {
			return true
		}
	}
	return false
}
