package wowsimclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/walkline/ToCloud9/apps/wowsimclient/orchestrator"
)

// Server is the HTTP API server for managing bot clients (node mode).
type Server struct {
	bots   sync.Map
	nextID atomic.Int64
	server *http.Server
}

// NewServer creates a new API server
func NewServer() *Server {
	return &Server{}
}

// LaunchRequest is the request body for launching a new bot.
type LaunchRequest struct {
	Username            string `json:"username"`
	Password            string `json:"password"`
	AuthServer          string `json:"auth_server"`
	CharacterName       string `json:"character_name"`
	RealmIndex          int    `json:"realm_index"`
	Race                uint8  `json:"race"`
	Class               uint8  `json:"class"`
	Gender              uint8  `json:"gender"`
	Mode                string `json:"mode"`
	DungeonName         string `json:"dungeon_name"`
	DataDir             string `json:"data_dir"`
	PathfindingAddr     string `json:"pathfinding_addr"`
	LuaScript           string `json:"lua_script"`
	BotID               string `json:"bot_id"`
	DeleteExistingChars bool   `json:"delete_existing_chars"`
	DisableTargetCache  bool   `json:"disable_target_cache"`
}

// LaunchResponse is the response body for a launch request.
type LaunchResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// LuaUpdateRequest is the request body for updating Lua code on a running bot.
type LuaUpdateRequest struct {
	BotID   string `json:"bot_id"`
	LuaCode string `json:"lua_code"`
}

// Start starts the HTTP server.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/launch", s.handleLaunch)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/status/all", s.handleStatusAll)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/lua", s.handleLuaUpdate)
	mux.HandleFunc("/events", s.handleEvents)

	s.server = &http.Server{Addr: addr, Handler: mux}
	fmt.Printf("Node server starting on %s\n", addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// LaunchFromOrchestrator starts a bot from an orchestrator request.
func (s *Server) LaunchFromOrchestrator(req orchestrator.NodeLaunchRequest) string {
	id := req.BotID
	if id == "" {
		id = fmt.Sprintf("bot-%d", s.nextID.Add(1))
	}

	fmt.Printf("[Node] LaunchFromOrchestrator: bot=%s char=%s race=%d class=%d delete=%v\n", id, req.CharacterName, req.Race, req.Class, req.DeleteExistingChars)

	config := BotConfig{
		Username:                 req.Username,
		Password:                 req.Password,
		AuthServer:               req.AuthServer,
		CharacterName:            req.CharacterName,
		Race:                     req.Race,
		Class:                    req.Class,
		Mode:                     req.Mode,
		DungeonName:              req.DungeonName,
		DataDir:                  req.DataDir,
		PathfindingAddress:       req.PathfindingAddr,
		LuaScript:                req.LuaScript,
		DeleteExistingCharacters: req.DeleteExistingChars,
		LogDecisionsToChat:       req.LogDecisionsToChat,
		DisableTargetCache:       req.DisableTargetCache,
	}

	bot := NewBot(id, config)
	s.bots.Store(id, bot)
	go bot.Run()
	return id
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

	id := req.BotID
	if id == "" {
		id = fmt.Sprintf("bot-%d", s.nextID.Add(1))
	}

	config := BotConfig{
		Username:                 req.Username,
		Password:                 req.Password,
		AuthServer:               req.AuthServer,
		CharacterName:            req.CharacterName,
		RealmIndex:               req.RealmIndex,
		Race:                     req.Race,
		Class:                    req.Class,
		Gender:                   req.Gender,
		Mode:                     req.Mode,
		DungeonName:              req.DungeonName,
		DataDir:                  req.DataDir,
		PathfindingAddress:       req.PathfindingAddr,
		LuaScript:                req.LuaScript,
		DeleteExistingCharacters: req.DeleteExistingChars,
		LogDecisionsToChat:       true,
		DisableTargetCache:       req.DisableTargetCache,
	}

	bot := NewBot(id, config)
	s.bots.Store(id, bot)

	fmt.Printf("[Node] handleLaunch: starting bot id=%s char=%s\n", id, req.CharacterName)
	go bot.Run()

	resp := LaunchResponse{ID: id, Message: "bot launched"}
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

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		// Stop all
		s.bots.Range(func(key, value interface{}) bool {
			bot := value.(*Bot)
			bot.Stop()
			// Stop() now also closes the conn to unblock readers immediately.
			return true
		})
		w.Write([]byte(`{"message":"all bots stopped"}`))
		return
	}

	val, ok := s.bots.Load(id)
	if !ok {
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}
	bot := val.(*Bot)
	bot.Stop()
	w.Write([]byte(fmt.Sprintf(`{"message":"bot %s stopped"}`, id)))
}

func (s *Server) handleLuaUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LuaUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.BotID != "" {
		val, ok := s.bots.Load(req.BotID)
		if !ok {
			http.Error(w, "bot not found", http.StatusNotFound)
			return
		}
		bot := val.(*Bot)
		if err := bot.LoadLuaScript(req.LuaCode); err != nil {
			http.Error(w, "lua error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Update all bots
		s.bots.Range(func(key, value interface{}) bool {
			bot := value.(*Bot)
			bot.LoadLuaScript(req.LuaCode)
			return true
		})
	}

	w.Write([]byte(`{"message":"lua updated"}`))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	w.Header().Set("Content-Type", "application/json")

	if id != "" {
		val, ok := s.bots.Load(id)
		if !ok {
			http.Error(w, "bot not found", http.StatusNotFound)
			return
		}
		bot := val.(*Bot)
		json.NewEncoder(w).Encode(bot.Events())
	} else {
		// All events from all bots
		allEvents := make(map[string][]BotEvent)
		s.bots.Range(func(key, value interface{}) bool {
			bot := value.(*Bot)
			allEvents[key.(string)] = bot.Events()
			return true
		})
		json.NewEncoder(w).Encode(allEvents)
	}
}
