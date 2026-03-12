package http_server

import (
	. "TaskOneUtils/configuration"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)


type Server struct {
	postHandlers map[string](func(http.ResponseWriter, *http.Request));
	getHandlers map[string](func(http.ResponseWriter, *http.Request));
	patchHandlers map[string](func(http.ResponseWriter, *http.Request));
	putHandlers map[string](func(http.ResponseWriter, *http.Request));
	deleteHandlers map[string](func(http.ResponseWriter, *http.Request));
	hostName string;
}

func NewServer(host string) *Server {
	return &Server {
		postHandlers: make(map[string](func(http.ResponseWriter, *http.Request))),
		getHandlers: make(map[string](func(http.ResponseWriter, *http.Request))),
		patchHandlers: make(map[string](func(http.ResponseWriter, *http.Request))),
		putHandlers: make(map[string](func(http.ResponseWriter, *http.Request))),
		deleteHandlers: make(map[string](func(http.ResponseWriter, *http.Request))),
		hostName: host,
	}
}

func (s *Server) RegisterHandler(method, path string, handler func(http.ResponseWriter, *http.Request)) {
	switch method {
	case http.MethodPost:
		s.postHandlers[path] = handler
	case http.MethodGet:
		s.getHandlers[path] = handler
	case http.MethodPatch:
		s.patchHandlers[path] = handler
	case http.MethodPut:
		s.putHandlers[path] = handler
	case http.MethodDelete:
		s.deleteHandlers[path] = handler
	}
}

func (s *Server) ServerLoop() error {
	stop := signalsSetup()

	if GlobalConfig == nil {
		return errors.New("Configuration is not initted")
	}

	mux := http.NewServeMux();

	var buildPath = func (method string, host string, path string) (string) {
		var path_builder strings.Builder;
		path_builder.WriteString(method);
		path_builder.WriteString(" /");
		path_builder.WriteString(host);
		path_builder.WriteString(path);
		return path_builder.String();
	}

	for path, handler := range s.postHandlers {
		fullPath := buildPath("POST", s.hostName, path)
		mux.HandleFunc(fullPath, handler)
		log.Printf("Register handler for path: %s\n", fullPath)
	}

	for path, handler := range s.getHandlers {
		fullPath := buildPath("GET", s.hostName, path)
		mux.HandleFunc(fullPath, handler)
		log.Printf("Register handler for path: %s\n", fullPath)
	}

	for path, handler := range s.patchHandlers {
		fullPath := buildPath("PATCH", s.hostName, path)
		mux.HandleFunc(fullPath, handler)
		log.Printf("Register handler for path: %s\n", fullPath)
	}

	for path, handler := range s.putHandlers {
		fullPath := buildPath("PUT", s.hostName, path)
		mux.HandleFunc(fullPath, handler)
		log.Printf("Register handler for path: %s\n", fullPath)
	}

	for path, handler := range s.deleteHandlers {
		fullPath := buildPath("DELETE", s.hostName, path)
		mux.HandleFunc(fullPath, handler)
		log.Printf("Register handler for path: %s\n", fullPath)
	}

	listen_addr, ok := GetConfParam[string](GlobalConfig, "listen_addr");
	port, ok := GetConfParam[int](GlobalConfig, "port");

	if !ok {
		return errors.New("Missing \"listen_addr\" or \"port\" parameters")
	}

	address := listen_addr + ":" + strconv.Itoa(port)

	serverError := make(chan error, 1)
	server := &http.Server{
		Addr: address,
		Handler: mux,
	}

	log.Println("Server started at  " + address)
	go serverRoutine(server, serverError)

	select {
	case <-stop:
		log.Println("Stopping signal requested")
	case err := <-serverError:
		log.Printf("Server error: %s\n", err)
	}

	// Start graceful shutdown
    log.Println("Server shutdown starting...")
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Can not stop server: %v\n", err)
    }
    
    log.Println("Server shutdown successfully!")

	return nil
}

func serverRoutine(server *http.Server, serverError chan error) {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		serverError <- err
	}
}

func signalsSetup() chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	return stop
}