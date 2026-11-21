package domain

import "errors"

type Document struct {
	ID    int64
	Title string
}

var ErrDocumentNotFound = errors.New("document not found")
