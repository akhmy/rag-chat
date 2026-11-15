package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"rag-chat/internal/appctx"
	"strconv"
	"strings"
)

type SessionService interface {
	Create(ctx context.Context, userID int64, title string) (int64, error)
}

type SessionHandler struct {
	svc SessionService
}

func NewSessionHandler(svc SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

type CreateSessionRequest struct {
	Title string `json:"title"`
}

func (r *CreateSessionRequest) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
}

var (
	ErrEmptyTitle = errors.New("empty title")
)

func (r *CreateSessionRequest) Validate() error {
	if r.Title == "" {
		return ErrEmptyTitle
	}

	return nil
}

type CreateSessionResponse struct {
	ID int64 `json:"id"`
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request, userID int64) {
	logger := appctx.Logger(r.Context())

	var req CreateSessionRequest
	if err := json.UnmarshalRead(r.Body, &req); err != nil {
		logger.Error("failed to unmarshal request body", "error", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	req.Normalize()
	if err := req.Validate(); err != nil {
		logger.Error("failed to validate request", "error", err)

		switch {
		case errors.Is(err, ErrEmptyTitle):
			http.Error(w, ErrEmptyTitle.Error(), http.StatusBadRequest)
		default:
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		}

		return
	}

	id, err := h.svc.Create(r.Context(), userID, req.Title)
	if err != nil {
		logger.Error("failed to create session", "error", err)
		return
	}

	respBody, err := json.Marshal(CreateSessionResponse{
		ID: id,
	})
	if err != nil {
		logger.Error("failed to marshal response body", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(respBody); err != nil {
		logger.Error("failed to write response body", "error", err)
		return
	}
}

var (
	ErrInvalidUserID = errors.New("invalid user id")
)

func (h *SessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := appctx.Logger(r.Context())

	value := r.PathValue("userID")
	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		logger.Error("failed to parse user id", "error", err)
		http.Error(w, ErrInvalidUserID.Error(), http.StatusBadRequest)
		return
	}

	if userID < 0 {
		logger.Error("negative user id")
		http.Error(w, ErrInvalidUserID.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.Create(w, r, userID)
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}
