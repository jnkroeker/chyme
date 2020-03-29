package ingest

import (
	"context"
	"encoding/json"
	"net/url"
	"net/http"
	"github.com/go-kit/kit/endpoint"
)

// gRPC requests

type IngestRequest struct {
	URL            string `json:"url"`
	Filter         string `json:"filter"`
	RecursionDepth int    `json:"recursionDepth"`
}

type IngestResponse struct {
	RES int64 `json:"res"`
	Err string `json:"err,omitempty"`
}

/** function names beginning with a lowercase letter are not exported from the package **/
func MakeIngestEndpoint(svc IngestService) endpoint.Endpoint {
	return func (_ context.Context, request interface{}) (interface{}, error) {
		req := request.(IngestRequest)
		url, err := url.Parse(req.URL)

		if err != nil {
			return IngestResponse{0, err.Error()}, err
		}
		res, err := svc.Ingest(url, req.Filter, req.RecursionDepth)
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