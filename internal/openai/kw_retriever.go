package openai

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

//go:embed kw_retriever_instructions
var keywordRetrieverInstructions string

type KeywordRetriever struct {
	client openai.Client
}

func NewKeywordRetriever(client openai.Client) *KeywordRetriever {
	return &KeywordRetriever{client: client}
}

func (kr *KeywordRetriever) RetrieveKeywords(ctx context.Context, query string) (string, error) {
	res, err := kr.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        openai.ChatModelGPT4oMini,
		Instructions: openai.String(keywordRetrieverInstructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(query),
		},
	})
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	return res.OutputText(), nil
}
