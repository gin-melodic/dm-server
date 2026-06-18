package dream

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
)

type sKnowledge struct{}

var shareKnowledge = &sKnowledge{}

// searchRequest represents the request structure for knowledge base search API
type searchRequest struct {
	Query   string   `json:"query"`
	Sources []string `json:"sources,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

// interpretDreamRequest represents the request structure for the L1-aware dream interpretation API.
type interpretDreamRequest struct {
	Description       string   `json:"description"`
	UserId            string   `json:"user_id,omitempty"`
	EmotionTags       []string `json:"emotion_tags,omitempty"`
	UseWeightedSearch bool     `json:"use_weighted_search"`
	SkipL1            bool     `json:"skip_l1,omitempty"`
}

// searchResponse represents the response structure from knowledge base search API
type searchResponseResult struct {
	Score               float64 `json:"score"`
	Source              string  `json:"source"`
	VectorScore         float64 `json:"vector_score"`
	ChineseKeywordBoost float64 `json:"chinese_keyword_boost"`
	StructuredBoost     float64 `json:"structured_boost"`
	TotalBoost          float64 `json:"total_boost"`
	Id                  string  `json:"id"`
	TextPreview         string  `json:"text_preview"`
	FullText            string  `json:"full_text"`
}

type searchResponse []searchResponseResult

type interpretDreamResponse struct {
	Success bool                       `json:"success"`
	Data    interpretDreamResponseData `json:"data"`
	Message string                     `json:"message"`
}

type interpretDreamResponseData struct {
	Interpretation  string                   `json:"interpretation"`
	InferenceLevel  string                   `json:"inference_level"`
	SymbolsDetected []string                 `json:"symbols_detected"`
	SymbolsMatched  []string                 `json:"symbols_matched"`
	FrameworksUsed  []string                 `json:"frameworks_used"`
	Results         map[string]frameworkData `json:"results"`
	TotalMatches    int                      `json:"total_matches"`
	SearchMethod    string                   `json:"search_method"`
}

type frameworkData struct {
	Passages []searchResponseResult `json:"passages"`
}

func (r *interpretDreamResponse) isL1Hit() bool {
	return r != nil && r.Success && r.Data.InferenceLevel == "L1" && r.Data.Interpretation != ""
}

func (r *interpretDreamResponse) toSearchResponse() *searchResponse {
	if r == nil || len(r.Data.Results) == 0 {
		return nil
	}

	results := make(searchResponse, 0, r.Data.TotalMatches)
	for _, framework := range r.Data.Results {
		results = append(results, framework.Passages...)
	}
	return &results
}

// getConfig Get knowledge base config
func (s *sKnowledge) getConfig(ctx context.Context) (map[string]string, error) {
	// Get knowledge base config
	knowledgeConfig, err := g.Cfg().Get(ctx, "knowledge")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get knowledge base config")
	}

	return knowledgeConfig.MapStrStr(), nil
}

func (s *sKnowledge) postJSON(ctx context.Context, path string, request any, responseBody any) error {
	knowledgeConfig, err := s.getConfig(ctx)
	if err != nil {
		return gerror.Wrap(err, "Failed to get knowledge base config")
	}

	baseURL := strings.TrimRight(knowledgeConfig["base_url"], "/")
	timeout := knowledgeConfig["timeout"]

	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}

	requestBody, err := gjson.Encode(request)
	if err != nil {
		return gerror.Wrap(err, "Failed to encode request")
	}

	url := fmt.Sprintf("%s%s", baseURL, path)
	glog.Infof(ctx, "Calling knowledge base: %s", url)
	client := g.Client().ContentJson()
	if timeout != "" {
		if t, err := gtime.ParseDuration(timeout + "s"); err == nil {
			client = client.Timeout(t)
		}
	}

	response, err := client.Post(ctx, url, requestBody)
	if err != nil {
		return gerror.Wrap(err, "Failed to send request to knowledge base service")
	}
	defer response.Close()

	body := response.ReadAll()
	if response.StatusCode != 200 {
		return gerror.Newf("Knowledge base API returned status %d: %s", response.StatusCode, string(body))
	}

	if err = gjson.DecodeTo(body, responseBody); err != nil {
		return gerror.Wrap(err, "Failed to parse response")
	}

	return nil
}

func (s *sKnowledge) interpretDream(ctx context.Context, query string, userId string, emotionTags []string) (*interpretDreamResponse, error) {
	request := interpretDreamRequest{
		Description:       query,
		UserId:            userId,
		EmotionTags:       emotionTags,
		UseWeightedSearch: true,
	}

	var resp interpretDreamResponse
	if err := s.postJSON(ctx, "/api/v1/interpret_dream", request, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// search Search knowledge base
func (s *sKnowledge) search(ctx context.Context, query string) (*searchResponse, error) {
	request := searchRequest{
		Query: query,
		Limit: 5,
	}

	var searchResp searchResponse
	if err := s.postJSON(ctx, "/api/v1/search", request, &searchResp); err != nil {
		return nil, err
	}

	return &searchResp, nil
}

// formatKnowledge Format searched knowledge into a string
func (s *sKnowledge) formatKnowledge(knowledge *searchResponse) string {
	if knowledge == nil || len(*knowledge) == 0 {
		return ""
	}

	result := "Related knowledge:\n"
	for i, item := range *knowledge {
		// Only get score >= 0.5
		if item.Score >= 0.5 {
			// title + full_text
			result += fmt.Sprintf("%d. %s\n%s\n", i+1, item.Source, item.FullText)
			result += "\n"
		}
	}

	return result
}
