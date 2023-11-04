package ollama

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/primalmotion/simplai/llm"
	"github.com/primalmotion/simplai/llm/internal/utils"
	"github.com/primalmotion/simplai/llm/ollama/internal/client"
	"github.com/primalmotion/simplai/utils/render"
)

// ollamaAPI is a ollama LLM implementation.
type ollamaAPI struct {
	client  *client.Client
	model   string
	options options
}

// New creates a new ollama LLM implementation.
func New(api string, model string, opts ...Option) (*ollamaAPI, error) { //nolint:revive

	url, err := url.Parse(api)
	if err != nil {
		return nil, fmt.Errorf("unable to parse url '%s': %w", api, err)
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	return &ollamaAPI{
		client:  client.NewClient(url),
		model:   model,
		options: o,
	}, nil
}

// Infer implemente the geneollamaclientrate interface for LLM.
func (o *ollamaAPI) Infer(ctx context.Context, prompt string, options ...llm.Option) (string, error) {

	opts := o.options.defaultInferenceConfig
	opts.Model = o.model
	opts.MaxTokens = utils.CountTokens(o.model, prompt)

	for _, opt := range options {
		opt(&opts)
	}

	ollamaOptions := o.options.ollamaOptions
	ollamaOptions.NumPredict = opts.MaxTokens
	ollamaOptions.Temperature = float32(opts.Temperature)
	ollamaOptions.Stop = opts.Stop
	ollamaOptions.TopK = opts.TopK
	ollamaOptions.TopP = float32(opts.TopP)
	ollamaOptions.Seed = opts.Seed
	ollamaOptions.RepeatPenalty = float32(opts.RepetitionPenalty)
	ollamaOptions.FrequencyPenalty = float32(opts.FrequencyPenalty)
	ollamaOptions.PresencePenalty = float32(opts.PresencePenalty)

	req := &client.GenerateRequest{
		Model:    opts.Model,
		System:   o.options.system,
		Prompt:   prompt,
		Template: o.options.customModelTemplate,
		Options:  ollamaOptions,
		Raw:      o.options.raw,
	}

	if opts.Debug {
		render.Box(fmt.Sprintf("[ollama-engine-request]\n\n%s", req), "4")
	}

	resp, err := o.client.Infer(ctx, req)
	if err != nil {
		return "", err
	}

	if opts.Debug {
		render.Box(fmt.Sprintf("[ollama-engine-response]\n\n%s", resp), "4")
	}

	return resp.Response, nil
}

// EmbedChunks implement the embeddings interface for chunks.
func (o *ollamaAPI) EmbedChunks(ctx context.Context, chunks []string, options ...llm.EmbeddingOption) ([][]float64, error) {

	opts := defaultEmbeddingConfig()
	for _, opt := range options {
		opt(&opts)
	}

	model := opts.Model
	if model == "" {
		model = o.model
	}

	emb := make([][]float64, 0, len(chunks))

	batches := utils.Batch(chunks, opts.BatchSize)
	for _, batch := range batches {

		currentEmbeddings := [][]float64{}

		for _, chunk := range chunks {
			req := &client.EmbeddingRequest{
				Prompt: chunk,
				Model:  model,
			}

			if opts.Debug {
				render.Box(fmt.Sprintf("[ollama-embedding-request]\n\n%s", req), "4")
			}

			embedding, err := o.client.Embed(ctx, req)
			if err != nil {
				return nil, err
			}

			if len(embedding.Embedding) == 0 {
				return nil, errors.New("no response")
			}

			if opts.Debug {
				render.Box(fmt.Sprintf("[ollama-embedding-response]\n\n%s", embedding), "4")
			}

			currentEmbeddings = append(currentEmbeddings, embedding.Embedding)
		}

		if len(chunks) != len(currentEmbeddings) {
			return currentEmbeddings, errors.New("no all input got emmbedded")
		}

		// get num of token in that batch
		// we should use the encoder of the model to get the tokens
		// but its not available. So we fall back on tiktoken
		numTokens := make([]float64, 0, len(batch))
		for _, text := range batch {
			numTokens = append(numTokens, float64(utils.CountTokens(opts.Model, text)))
		}

		if len(currentEmbeddings) > 1 {
			combinedVectors, err := utils.CombineBatchedEmbedding(currentEmbeddings, numTokens)
			if err != nil {
				return [][]float64{}, err
			}
			emb = append(emb, combinedVectors)
			continue
		}

		emb = append(emb, currentEmbeddings...)
	}

	return emb, nil
}

// EmbedQuery implement the embeddings interface for query.
func (o *ollamaAPI) EmbedQuery(ctx context.Context, query string, options ...llm.EmbeddingOption) ([]float64, error) {
	c, err := o.EmbedChunks(ctx, []string{query}, options...)
	if err != nil {
		return nil, err
	}
	return c[0], nil
}
