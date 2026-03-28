package transport

import (
	"fmt"
	"log"
	"net/http"
)

type Server struct {
	mux  *http.ServeMux
	addr string
}

func NewServer(addr string) *Server {
	return &Server{
		mux:  http.NewServeMux(),
		addr: addr,
	}
}


func (s *Server) Register(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
	log.Printf("[transport] registered route %s", pattern)
}

func (s *Server) Start() error {
	log.Printf("[transport] listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) BaseURL() string {
	return fmt.Sprintf("http://%s", s.addr)
}