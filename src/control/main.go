package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/tntmeijs/invokex/control/server"
)

type (
	sourceCodeLanguage = string

	deleteSourceCodePayload struct {
		Name string `json:"name"`
	}

	messageResponseBody struct {
		Message string `json:"message"`
	}
)

const (
	sourceCodeLanguageGo sourceCodeLanguage = "GOLANG"

	sourceCodeDestination   string = "./.sourcecode"
	sourceCodeFileExtension string = "zip"

	kilobyte          int64 = 1024
	megabyte          int64 = kilobyte * kilobyte
	maxSourceCodeSize int64 = 50 * megabyte
)

var supportedSourceCodeLanguages = []sourceCodeLanguage{
	sourceCodeLanguageGo,
}

func uploadSourceCode(r server.Request) (server.Response, error) {
	if contentType := r.Raw.Header.Get("Content-Type"); !strings.Contains(contentType, "multipart/form-data") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: fmt.Sprintf("user has not uploaded multipart form data, got: %s", contentType)},
		}, nil
	}

	language := strings.ToUpper(strings.TrimSpace(r.Raw.FormValue("sourceCodeLanguage")))
	if len(language) == 0 {
		fmt.Println("user did not specify the source code language")

		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: "user did not specify the source code language"},
		}, nil
	}

	if !slices.Contains(supportedSourceCodeLanguages, language) {
		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: fmt.Sprintf("source code language %s is not supported", language)},
		}, nil
	}

	fmt.Printf("user has uploaded source code of type %s\n", language)

	file, header, err := r.Raw.FormFile("sourceCode")
	if err != nil {
		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: fmt.Sprintf("no source code found: %v", err)},
		}, nil
	}

	if header.Size > maxSourceCodeSize {
		return server.Response{
			StatusCode: http.StatusRequestEntityTooLarge,
			Body:       messageResponseBody{Message: fmt.Sprintf("source code package is too big, max size: %dMB", maxSourceCodeSize/megabyte)},
		}, nil
	}

	parts := strings.Split(header.Filename, ".")
	if len(parts) == 0 {
		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: "invalid file - no extension found"},
		}, nil
	}

	extension := strings.ToLower(parts[len(parts)-1])
	if extension != sourceCodeFileExtension {
		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: fmt.Sprintf("invalid file extension, expected .%s got .%s", sourceCodeFileExtension, extension)},
		}, nil
	}

	defer func() {
		if err := file.Close(); err != nil {
			panic(fmt.Sprintf("could not close file: %v", err))
		}
	}()

	var buffer = make([]byte, 0, header.Size)
	var tmp = make([]byte, kilobyte)

	for err == nil {
		_, err = file.Read(tmp)
		buffer = append(buffer, tmp...)
	}

	if err != io.EOF {
		return server.Response{}, fmt.Errorf("failed to read file: %v", err)
	}

	// TODO: generate custom files names and handle conflict resolution gracefully - right now we simply override.
	if err = os.WriteFile(fmt.Sprintf("%s/%s", sourceCodeDestination, header.Filename), buffer[:header.Size], 0200); err != nil {
		return server.Response{}, fmt.Errorf("could not create file for source code: %v", err)
	}

	return server.Response{
		StatusCode: http.StatusOK,
		Body:       messageResponseBody{Message: fmt.Sprintf("file %s uploaded successfully", header.Filename)},
	}, nil
}

func deleteSourceCode(r server.Request) (server.Response, error) {
	if contentType := r.Raw.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: fmt.Sprintf("user has not uploaded json data, got: %s", contentType)},
		}, nil
	}

	payload := deleteSourceCodePayload{}
	if err := json.NewDecoder(r.Raw.Body).Decode(&payload); err != nil {
		return server.Response{}, fmt.Errorf("could not decode payload: %v", err)
	}

	fileName := payload.Name

	if len(fileName) == 0 {
		return server.Response{
			StatusCode: http.StatusBadRequest,
			Body:       messageResponseBody{Message: "no file name has been specified"},
		}, nil
	}

	if !strings.HasSuffix(fileName, ".zip") {
		fileName += ".zip"
	}

	if err := os.Remove(fmt.Sprintf("%s/%s", sourceCodeDestination, fileName)); err != nil {
		return server.Response{
			StatusCode: http.StatusNotFound,
			Body:       messageResponseBody{Message: fmt.Sprintf("no source code with name %s was found", fileName)},
		}, nil
	}

	fmt.Printf("deleted file %s\n", fileName)
	return server.Response{StatusCode: http.StatusNoContent}, nil
}

func main() {
	// Source code will be place here
	if err := os.MkdirAll(sourceCodeDestination, 0600); err != nil {
		panic(fmt.Sprintf("could not create source code directory: %v", err))
	}

	err := server.NewHttpServer().
		RegisterRoute(server.HttpPost, "/api/v1/sourcecode", uploadSourceCode).
		RegisterRoute(server.HttpDelete, "/api/v1/sourcecode", deleteSourceCode).
		Run(":8080")

	if err != nil {
		panic(fmt.Sprintf("server has closed: %s", err.Error()))
	}
}
