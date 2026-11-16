package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	_ "embed"
)

type TitleGenerator struct {
	client openai.Client
}

//go:embed title_gen_instructions
var titleGeneratorInstructions string

func NewTitleGenerator(client openai.Client) *TitleGenerator {
	return &TitleGenerator{client: client}
}

func (gen *TitleGenerator) GenerateTitle(ctx context.Context, query string) (string, error) {
	res, err := gen.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        openai.ChatModelGPT4oMini,
		Instructions: openai.String(titleGeneratorInstructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(query),
		},
	})
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	title := strings.TrimSpace(res.OutputText())
	if title == "" {
		return "", errors.New("empty title")
	}

	return res.OutputText(), nil
}
