// PicoClaw - Ultra-lightweight personal AI agent
// WebUI Server for chat interface
//
// Copyright (c) 2026 PicoClaw contributors

package webui

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	server    *http.Server
	mu        sync.RWMutex
	handlers  *Handlers
	startTime time.Time
	config    *Config
}

type Config struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

func NewServer(cfg *Config, handlers *Handlers) *Server {
	mux := http.NewServeMux()
	s := &Server{
		handlers:  handlers,
		startTime: time.Now(),
		config:    cfg,
	}

	// API routes
	mux.HandleFunc("/api/chat", handlers.ChatHandler)
	mux.HandleFunc("/api/sessions", handlers.SessionsHandler)
	mux.HandleFunc("/api/history", handlers.HistoryHandler)
	mux.HandleFunc("/api/ready", handlers.ReadyHandler)

	// Static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) StartContext(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.server.Shutdown(context.Background())
	}
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
