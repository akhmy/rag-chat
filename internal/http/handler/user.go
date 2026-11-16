package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
)

type UserService interface {
	CreateUser(ctx context.Context, telegramID *int64) (domain.User, error)
}

type SignUpResponse struct {
	ID int64 `json:"id"`
}

func SignUp(userSvc UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := appctx.Logger(r.Context())

		user, err := userSvc.CreateUser(r.Context(), nil)
		if err != nil {
			logger.Error("failed to create user", "error", err)

			switch {
			case errors.Is(err, domain.ErrUserAlreadyExists):
				http.Error(w, domain.ErrUserAlreadyExists.Error(), http.StatusConflict)
			default:
				http.Error(w, responseInternalServerError, http.StatusInternalServerError)
			}

			return
		}

		respBody, err := json.Marshal(SignUpResponse{ID: user.ID})
		if err != nil {
			logger.Error("failed to marshal response", "error", err)
			http.Error(w, responseInternalServerError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write(respBody); err != nil {
			logger.Error("failed to write response", "error", err)
			return
		}
	}
}
