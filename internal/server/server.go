package server

import (
	"net/http"
)

type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func New(registrars ...RouteRegistrar) *http.ServeMux {
	mux := http.NewServeMux()
	for _, registrar := range registrars {
		registrar.RegisterRoutes(mux)
	}
	return mux
}
