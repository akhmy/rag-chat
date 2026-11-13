package middleware

import "net/http"

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
