package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/roommatch/internal/infrai"
	"example.com/roommatch/internal/matchmaker"
)

type server struct{ api *infrai.Client }

type playerTokens struct {
	PlayerID      string `json:"player_id"`
	RealtimeToken string `json:"realtime_token"`
	RTCToken      string `json:"rtc_token"`
}

type matchResponse struct {
	matchmaker.Decision
	Tokens []playerTokens `json:"tokens,omitempty"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	s := server{api: infrai.New(key)}
	mux.HandleFunc("POST /matches", s.createMatch)
	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("roommatch listening on %s", addr)
	log.Fatal(httpServer.ListenAndServe())
}

func (s server) createMatch(w http.ResponseWriter, r *http.Request) {
	var input matchmaker.Request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	decision, err := matchmaker.Decide(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if decision.State == "moderation_queued" {
		writeJSON(w, http.StatusAccepted, matchResponse{Decision: decision})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.api.CreateChannel(ctx, decision.Room, input.RequestID); err != nil {
		s.writeAPIError(w, err)
		return
	}
	if err := s.api.CreateRoom(ctx, decision.Room, input.Region, input.RequestID, len(input.Players)); err != nil {
		s.writeAPIError(w, err)
		return
	}
	tokens := make([]playerTokens, 0, len(input.Players))
	for _, player := range input.Players {
		realtimeToken, err := s.api.IssueRealtimeToken(ctx, player.ID, decision.Room, input.RequestID+":"+player.ID)
		if err != nil {
			s.writeAPIError(w, err)
			return
		}
		rtcToken, err := s.api.IssueRTCToken(ctx, decision.Room, player.ID, player.DisplayName, input.RequestID+":"+player.ID)
		if err != nil {
			s.writeAPIError(w, err)
			return
		}
		tokens = append(tokens, playerTokens{PlayerID: player.ID, RealtimeToken: realtimeToken, RTCToken: rtcToken})
	}
	if err := s.api.PublishReady(ctx, decision.Room, input.RequestID, map[string]any{"room": decision.Room, "players": len(input.Players)}); err != nil {
		s.writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, matchResponse{Decision: decision, Tokens: tokens})
}

func (server) writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
