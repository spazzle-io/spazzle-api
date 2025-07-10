package middleware

// Headers
const (
	userAgentHeader              = "user-agent"
	contentTypeHeader            = "Content-Type"
	applicationJSONValue         = "application/json"
	xForwardedForHeader          = "x-forwarded-for"
	grpcGatewayUserAgentHeader   = "grpcgateway-user-agent"
	xServiceAuthenticationHeader = "x-service-authentication"
	xRateLimitLimitHeader        = "x-ratelimit-limit"
	xRateLimitRemainingHeader    = "x-ratelimit-remaining"
	xRateLimitResetHeader        = "x-ratelimit-reset"
)

// Errors
const (
	InternalServerError             string = "An unexpected error occurred while processing your request"
	RateLimitExceededError          string = "Slow down. Too many requests. Try again shortly"
	MissingXForwardedForHeaderError string = "X-Forwarded-For header is required for accurate processing"
)

type ReqContextKey string

const (
	ClientIP              ReqContextKey = "client_ip"
	UserAgent             ReqContextKey = "user_agent"
	ServiceAuthentication ReqContextKey = "service_authentication"
	AuthenticatedService  ReqContextKey = "authenticated_service"
)
