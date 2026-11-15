package handler

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"rag-chat/internal/appctx"
	"rag-chat/internal/domain"
)

type CreateUserUseCase interface {
	Execute(ctx context.Context, telegramID *int64) (domain.User, error)
}

type SignUpRequest struct {
	TelegramID *int64 `json:"telegram_id"`
}

var ErrNegativeTelegramID = errors.New("negative Telegram ID")

func (req SignUpRequest) Validate() error {
	if req.TelegramID != nil {
		if *req.TelegramID < 0 {
			return ErrNegativeTelegramID
		}
	}

	return nil
}

type SignUpResponse struct {
	ID int64 `json:"id"`
}

func SignUp(uc CreateUserUseCase) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := appctx.Logger(r.Context())

		var req SignUpRequest
		if err := json.UnmarshalRead(r.Body, &req); err != nil {
			logger.Error("failed to unmarshal request body", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if err := req.Validate(); err != nil {
			logger.Error("failed to validate request", "error", err)

			switch {
			case errors.Is(err, ErrNegativeTelegramID):
				http.Error(w, ErrNegativeTelegramID.Error(), http.StatusBadRequest)
			default:
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			}

			return
		}

		user, err := uc.Execute(r.Context(), req.TelegramID)
		if err != nil {
			logger.Error("failed to execute usecase", "error", err)

			switch {
			case errors.Is(err, domain.ErrUserAlreadyExists):
				http.Error(w, domain.ErrUserAlreadyExists.Error(), http.StatusConflict)
			default:
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			return
		}

		respBody, err := json.Marshal(SignUpResponse{ID: user.ID})
		if err != nil {
			logger.Error("failed to marshal response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write(respBody); err != nil {
			logger.Error("failed to write response", "error", err)
			return
		}
	})
}
