package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"rag-chat/internal/appctx"
	"strconv"
	"strings"
	"time"
)

type AskUseCase interface {
	Execute(ctx context.Context, sessionID int64, query string, limit int) (string, error)
}

type AskRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

var (
	ErrEmptyQuery       = errors.New("empty query")
	ErrQueryTooLarge    = errors.New("query too large")
	ErrNegativeLimit    = errors.New("limit not positive")
	ErrInvalidSessionID = errors.New("invalid session ID")
)

func (req AskRequest) Validate() error {
	// Placeholder value.
	if len(req.Query) > 2048 {
		return ErrQueryTooLarge
	}

	if strings.TrimSpace(req.Query) == "" {
		return ErrEmptyQuery
	}

	if req.Limit == 0 {
		return ErrNegativeLimit
	}

	return nil
}

type Response struct {
	Reply string `json:"reply"`
}

func Ask(uc AskUseCase) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := appctx.Logger(r.Context())

		value := r.PathValue("sessionID")
		sessionID, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			logger.Error("failed to parse session ID")
			http.Error(w, ErrInvalidSessionID.Error(), http.StatusBadRequest)
			return
		}

		req := AskRequest{}
		err = json.UnmarshalRead(r.Body, &req)
		if err != nil {
			logger.Error("failed to unmarshal request", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if err := req.Validate(); err != nil {
			logger.Info("failed to validate request", "error", err)

			switch {
			case errors.Is(err, ErrEmptyQuery):
				http.Error(w, ErrEmptyQuery.Error(), http.StatusBadRequest)
			case errors.Is(err, ErrQueryTooLarge):
				http.Error(w, ErrQueryTooLarge.Error(), http.StatusBadRequest)
			case errors.Is(err, ErrNegativeLimit):
				http.Error(w, ErrNegativeLimit.Error(), http.StatusBadRequest)
			default:
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			}

			return
		}

		usecaseCtx, cancel := context.WithTimeout(r.Context(), time.Second*15)
		defer cancel()

		reply, err := uc.Execute(usecaseCtx, sessionID, req.Query, req.Limit)
		if err != nil {
			logger.Error("failed to execute ask", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		body, err := json.Marshal(Response{Reply: reply})
		if err != nil {
			logger.Error("failed to marshal response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(body)
		if err != nil {
			logger.Error("failed to write response", "error", err)
			return
		}
	})
}
