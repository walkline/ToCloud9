package wowsimclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

// Server is the HTTP API server for managing bot clients
type Server struct {
	bots   sync.Map
	nextID atomic.Int64
}

// NewServer creates a new API server
func NewServer() *Server {
	return &Server{}
}

// LaunchRequest is the request body for launching a new bot
type LaunchRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	AuthServer    string `json:"auth_server"`
	CharacterName string `json:"character_name"`
	RealmIndex    int    `json:"realm_index"`
	Race          uint8  `json:"race"`
	Class         uint8  `json:"class"`
	Gender        uint8  `json:"gender"`
}

// LaunchResponse is the response body for a launch request
type LaunchResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/launch", s.handleLaunch)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/status/all", s.handleStatusAll)

	fmt.Printf("Starting HTTP server on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LaunchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" || req.AuthServer == "" || req.CharacterName == "" {
		http.Error(w, "username, password, auth_server, and character_name are required", http.StatusBadRequest)
		return
	}

	id := fmt.Sprintf("bot-%d", s.nextID.Add(1))

	config := BotConfig{
		Username:      req.Username,
		Password:      req.Password,
		AuthServer:    req.AuthServer,
		CharacterName: req.CharacterName,
		RealmIndex:    req.RealmIndex,
		Race:          req.Race,
		Class:         req.Class,
		Gender:        req.Gender,
	}

	bot := NewBot(id, config)
	s.bots.Store(id, bot)

	// Launch bot in background
	go func() {
		bot.Run()
	}()

	resp := LaunchResponse{
		ID:      id,
		Message: "bot launched",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id query parameter required", http.StatusBadRequest)
		return
	}

	val, ok := s.bots.Load(id)
	if !ok {
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}

	bot := val.(*Bot)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot.Status())
}

func (s *Server) handleStatusAll(w http.ResponseWriter, r *http.Request) {
	var results []BotResult
	s.bots.Range(func(key, value interface{}) bool {
		bot := value.(*Bot)
		results = append(results, bot.Status())
		return true
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
