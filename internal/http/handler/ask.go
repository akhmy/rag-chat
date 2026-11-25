package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"iter"
	"net/http"
	"rag-chat/internal/appctx"
	"rag-chat/internal/usecase"
	"strconv"
	"strings"
	"time"
)

type AskRequest struct {
	Message string `json:"message"`
}

func (req *AskRequest) Normalize() {
	req.Message = strings.TrimSpace(req.Message)
}

var (
	ErrEmptyMessage    = errors.New("empty message")
	ErrMessageTooLarge = errors.New("message too large")
)

func (req *AskRequest) Validate(maxQueryLength int) error {
	switch {
	case len(req.Message) > maxQueryLength:
		return ErrMessageTooLarge
	case req.Message == "":
		return ErrEmptyMessage
	default:
		return nil
	}
}

type AskResponse struct {
	Message string `json:"message"`
}

type AskUseCase interface {
	Execute(ctx context.Context, in *usecase.AskInput) iter.Seq2[string, error]
}

type AskHandlerConfig struct {
	Timeout        time.Duration
	MaxQueryLength int
	RetrieveLimit  int
}

type AskHandler struct {
	ask    AskUseCase
	config AskHandlerConfig
}

func NewAskHandler(ask AskUseCase, cfg AskHandlerConfig) *AskHandler {
	return &AskHandler{
		ask:    ask,
		config: cfg,
	}
}

var (
	ErrInvalidSessionID = errors.New("invalid session ID")
)

func (h *AskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := appctx.Logger(r.Context())

	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		logger.Error("failed to parse userID")
		http.Error(w, ErrInvalidUserID.Error(), http.StatusBadRequest)
		return
	}

	var req AskRequest
	err = json.UnmarshalRead(r.Body, &req)
	if err != nil {
		logger.Error("failed to unmarshal request", "error", err)
		http.Error(w, responseBadRequest, http.StatusBadRequest)
		return
	}

	if err := req.Validate(h.config.MaxQueryLength); err != nil {
		logger.Info("failed to validate request", "error", err)

		switch {
		case errors.Is(err, ErrEmptyMessage):
			http.Error(w, ErrEmptyMessage.Error(), http.StatusBadRequest)
		case errors.Is(err, ErrMessageTooLarge):
			http.Error(w, ErrMessageTooLarge.Error(), http.StatusBadRequest)
		default:
			http.Error(w, responseBadRequest, http.StatusBadRequest)
		}

		return
	}

	askCtx, cancel := context.WithTimeout(r.Context(), h.config.Timeout)
	defer cancel()

	var message strings.Builder
	for delta, err := range h.ask.Execute(askCtx, &usecase.AskInput{
		UserID:     userID,
		Query:      req.Message,
		TurnLimit:  10,
		ChunkLimit: h.config.RetrieveLimit,
	}) {
		if err != nil {
			logger.Error("failed to execute ask", "error", err)
			http.Error(w, responseInternalServerError, http.StatusInternalServerError)
			return
		}
		message.WriteString(delta)
	}

	body, err := json.Marshal(AskResponse{Message: message.String()})
	if err != nil {
		logger.Error("failed to marshal response", "error", err)
		http.Error(w, responseInternalServerError, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(body)
	if err != nil {
		logger.Error("failed to write response", "error", err)
		return
	}
}
