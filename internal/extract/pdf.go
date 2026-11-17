package extract

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"rag-chat/internal/appctx"
	"strings"
)

type PDFSource struct {
	path  string
	title string
}

const pdfHeader = "%PDF-"

var ErrIsNotPDF = fmt.Errorf("pdf source is not a PDF file")

func NewPDFSource(path string, title string) (*PDFSource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	var header [5]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if string(header[:]) != pdfHeader {
		return nil, ErrIsNotPDF
	}

	return &PDFSource{path: path, title: title}, nil
}

func (s *PDFSource) Title() string {
	return s.title
}

func (s *PDFSource) Iter(ctx context.Context) iter.Seq2[string, error] {
	logger := appctx.Logger(ctx)

	return func(yield func(string, error) bool) {
		cmdCtx, cancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(cmdCtx, "pdftotext", s.path, "-")
		defer cancel()

		out, err := cmd.StdoutPipe()
		if err != nil {
			yield("", fmt.Errorf("stdout pipe: %w", err))
			return
		}

		if err := cmd.Start(); err != nil {
			yield("", fmt.Errorf("start pdftotext: %w", err))
			return
		}

		reader := bufio.NewReader(out)
		for page := 1; ; page++ {
			text, err := reader.ReadString('\f')
			if err != nil && err != io.EOF {
				cancel()

				if err := cmd.Wait(); err != nil {
					logger.Warn("failed to wait for pdftotext: %w", "error", err)
				}

				yield("", fmt.Errorf("read page %d: %w", page, err))

				return
			}

			text = strings.TrimSpace(text)
			if text != "" {
				if !yield(text, nil) {
					cancel()

					if err := cmd.Wait(); err != nil {
						logger.Warn("failed to wait for pdftotext: %w", "error", err)
					}

					return
				}
			}

			if err == io.EOF {
				break
			}
		}

		if err := cmd.Wait(); err != nil {
			yield("", fmt.Errorf("wait pdftotext: %w", err))
		}
	}
}
