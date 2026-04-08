package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type (
	// Request represents an incoming HTTP request.
	Request struct {
		Raw *http.Request
	}

	// Response represents an HTTP response.
	Response struct {
		StatusCode int
		Body       any
	}

	// HttpServer launches an HTTP server.
	HttpServer struct {
		handler *http.ServeMux
	}

	HttpMethod  string
	HandlerFunc func(Request) (Response, error)

	internalServerErrorBody struct {
		ErrorMessage string `json:"error"`
	}
)

const DefaultPort int = 8080

const (
	HttpGet     HttpMethod = "GET"
	HttpPost    HttpMethod = "POST"
	HttpPatch   HttpMethod = "PATCH"
	HttpDelete  HttpMethod = "DELETE"
	HttpOptions HttpMethod = "OPTIONS"
	HttpHead    HttpMethod = "HEAD"
)

// NewHttpServer creates a new HttpServer.
func NewHttpServer() *HttpServer {
	return &HttpServer{
		handler: http.NewServeMux(),
	}
}

// ReturnResponse is a helper function to return a Response from a HandlerFunc.
func ReturnResponse(statusCode int, body ...any) (Response, error) {
	r := Response{StatusCode: statusCode}

	if len(body) > 0 {
		r.Body = body[0]
	}

	return r, nil
}

// ReturnError is a helper function to return an error from a HandlerFunc.
func ReturnError(err error) (Response, error) {
	return Response{}, err
}

// RegisterRoute adds a new endpoint handler for the specified path pattern.
func (s *HttpServer) RegisterRoute(method HttpMethod, path string, handler HandlerFunc) *HttpServer {
	s.handler.HandleFunc(fmt.Sprintf("%s %s", method, path), func(w http.ResponseWriter, r *http.Request) {
		request := Request{Raw: r}
		response, err := handler(request)

		// Handler did not complete successfully.
		// No matter what the response is, we will always return an internal server error because handlers should always complete without errors.
		if err != nil {
			response.StatusCode = http.StatusInternalServerError
			response.Body = internalServerErrorBody{ErrorMessage: err.Error()}
		}

		w.WriteHeader(response.StatusCode)

		if response.Body != nil {
			var bodyBytes []byte
			bodyBytes, err = json.Marshal(response.Body)

			if err != nil {
				// Unrecoverable.
				panic(fmt.Sprintf("unable to construct response body: %s", err.Error()))
			}

			if _, err = w.Write(bodyBytes); err != nil {
				// Unrecoverable.
				panic(fmt.Sprintf("unable to write response body: %s", err.Error()))
			}
		}
	})

	return s
}

// Run makes HttpServer start listening on host:port.
func (s *HttpServer) Run(address string) error {
	instance := http.Server{
		Handler: s.handler,
		Addr:    address,
	}

	return instance.ListenAndServe()
}
