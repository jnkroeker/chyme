package ingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-kit/kit/endpoint"
	"kroekerlabs.dev/chyme/services/internal/core"
)

// gRPC requests

type IngestRequest struct {
	URL            string `json:"url"`
	Filter         string `json:"filter"`
	RecursionDepth int    `json:"recursionDepth"`
}

type IngestResponse struct {
	RES int    `json:"res"`
	Err string `json:"err,omitempty"`
}

// The empty interface request parameter and response on the return function value is the signiture of a go-kit endpoint
func MakeIngestEndpoint(svc IngestService) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(IngestRequest)
		sourceUrl, err := url.Parse(req.URL)

		if err != nil {
			return IngestResponse{0, err.Error()}, err
		}
		res, err := svc.Ingest(&core.Resource{Url: sourceUrl}, req.Filter, req.RecursionDepth)
		if err != nil {
			return IngestResponse{res, err.Error()}, nil
		}
		return IngestResponse{res, ""}, nil
	}
}

func DecodeIngestRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func EncodeIngestResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}
