package dream

import (
	"context"
	"fmt"

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

// getConfig Get knowledge base config
func (s *sKnowledge) getConfig(ctx context.Context) (map[string]string, error) {
	// Get knowledge base config
	knowledgeConfig, err := g.Cfg().Get(ctx, "knowledge")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get knowledge base config")
	}

	return knowledgeConfig.MapStrStr(), nil
}

// search Search knowledge base
func (s *sKnowledge) search(ctx context.Context, query string) (*searchResponse, error) {
	// Get knowledge base config
	knowledgeConfig, err := s.getConfig(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get knowledge base config")
	}

	baseURL := knowledgeConfig["base_url"]
	timeout := knowledgeConfig["timeout"]

	// Set default values
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}

	// Construct request
	request := searchRequest{
		Query: query,
		Limit: 5,
	}

	requestBody, err := gjson.Encode(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to encode request")
	}

	// Send POST request to knowledge base service
	url := fmt.Sprintf("%s/api/v1/search", baseURL)
	glog.Infof(ctx, "Searching knowledge base: %s", url)
	client := g.Client()
	if timeout != "" {
		if t, err := gtime.ParseDuration(timeout + "s"); err == nil {
			client = client.Timeout(t)
		}
	}

	response, err := client.Post(ctx, url, requestBody)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to send request to knowledge base service")
	}
	defer response.Close()

	// Check HTTP status code
	if response.StatusCode != 200 {
		return nil, gerror.Newf("Knowledge base API returned status %d: %s", response.StatusCode, response.ReadAllString())
	}

	// Parse response
	var searchResp searchResponse
	err = gjson.DecodeTo(response.ReadAll(), &searchResp)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to parse response")
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
