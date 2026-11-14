package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"rag-chat/internal/appcontext"
	"strings"
	"time"
)

type AskPipeline interface {
	Execute(ctx context.Context, query string, limit int) (string, error)
}

type AskRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

var (
	ErrEmptyQuery    = errors.New("empty query")
	ErrQueryTooLarge = errors.New("query too large")
	ErrNegativeLimit = errors.New("limit not positive")
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

func Ask(pl AskPipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := appcontext.Logger(r.Context())

		req := AskRequest{}
		err := json.UnmarshalRead(r.Body, &req)
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

		pipelineCtx, cancel := context.WithTimeout(r.Context(), time.Second*15)
		defer cancel()

		reply, err := pl.Execute(pipelineCtx, req.Query, req.Limit)
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
	}
}
