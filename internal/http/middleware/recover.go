package middleware

import "net/http"

const responseInternalServerError = "Internal Server Error"

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(responseInternalServerError))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
