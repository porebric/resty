package resty

import (
	"net/http"

	"github.com/rs/cors"
)

var corsAllowedOrigins []string
var corsAllowedMethods []string
var corsAllowedHeaders []string

func SetCors(allowedOrigins, allowedMethods, allowedHeaders []string) {
	corsAllowedOrigins = allowedOrigins
	corsAllowedMethods = allowedMethods
	corsAllowedHeaders = allowedHeaders
}

func setCors() http.Handler {
	if len(corsAllowedOrigins) == 0 || len(corsAllowedMethods) == 0 || len(corsAllowedHeaders) == 0 {
		return http.DefaultServeMux
	}
	co := cors.New(cors.Options{
		AllowedOrigins:   corsAllowedOrigins,
		AllowedMethods:   corsAllowedMethods,
		AllowedHeaders:   corsAllowedHeaders,
		AllowCredentials: true,
	})

	return co.Handler(http.DefaultServeMux)
}
