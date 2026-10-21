package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Mimic890/kuraban/internal/state"

	"github.com/Noooste/azuretls-client"
)

// EndpointCheck is the outcome of probing one endpoint of a cluster.
type EndpointCheck struct {
	OK      bool   `json:"ok"`
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// ConnectionCheck is what the "Test connection" button shows.
type ConnectionCheck struct {
	OK         bool          `json:"ok"`
	APIVersion string        `json:"api_version,omitempty"`
	Admin      EndpointCheck `json:"admin"`
	S3         EndpointCheck `json:"s3"`
}

// CheckConnection probes the admin API (v2, then v1) with the given token and
// makes one request to the S3 endpoint. Unlike NewAdminService it does not
// retry, so the UI gets an answer within a few seconds.
func CheckConnection(ctx context.Context, cfg state.ClusterConfig) ConnectionCheck {
	out := ConnectionCheck{}

	s3URL := cfg.Endpoint
	if !strings.Contains(s3URL, "://") {
		scheme := "http://"
		if cfg.UseSSL {
			scheme = "https://"
		}
		s3URL = scheme + s3URL
	}
	// Any HTTP answer (typically 403 AccessDenied without credentials) proves
	// the S3 API is listening.
	s3Done := make(chan EndpointCheck, 1)
	go func() {
		if status, err := probeStatus(ctx, strings.TrimRight(s3URL, "/")+"/", ""); err != nil {
			s3Done <- EndpointCheck{Message: describeDialError(err)}
		} else {
			s3Done <- EndpointCheck{OK: true, Status: status}
		}
	}()

	status, err := probeStatus(ctx, cfg.AdminEndpoint+"/v2/GetClusterHealth", cfg.AdminToken)
	if err == nil && status == http.StatusNotFound {
		if s, e := probeStatus(ctx, cfg.AdminEndpoint+"/v1/health", cfg.AdminToken); e == nil && s >= 200 && s < 300 {
			out.APIVersion = "v1"
			status = s
		}
	} else if err == nil && status >= 200 && status < 300 {
		out.APIVersion = "v2"
	}
	switch {
	case err != nil:
		out.Admin = EndpointCheck{Message: describeDialError(err)}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		out.Admin = EndpointCheck{Status: status, Message: "admin token rejected"}
	case status < 200 || status >= 300:
		out.Admin = EndpointCheck{Status: status, Message: fmt.Sprintf("unexpected HTTP %d (is this the admin API port, usually 3903?)", status)}
	default:
		out.Admin = EndpointCheck{OK: true, Status: status}
	}

	out.S3 = <-s3Done
	out.OK = out.Admin.OK && out.S3.OK
	return out
}

func probeStatus(ctx context.Context, url, token string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	session := azuretls.NewSession()
	defer session.Close()
	req := &azuretls.Request{Method: http.MethodGet, Url: url, IgnoreBody: true}
	if token != "" {
		req.OrderedHeaders = azuretls.OrderedHeaders{{"Authorization", "Bearer " + token}}
	}
	resp, err := session.Do(req, ctx)
	if err != nil {
		return 0, err
	}
	resp.RawBody.Close()
	return resp.StatusCode, nil
}

func describeDialError(err error) string {
	msg := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timed out (host unreachable or port filtered)"
	case strings.Contains(msg, "connection refused"):
		return "connection refused (nothing listens on that port)"
	case strings.Contains(msg, "no such host"):
		return "host not found (DNS)"
	}
	return msg
}
