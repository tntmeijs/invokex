package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
)

type (
	sourceCodeLanguage = string

	deleteSourceCodePayload struct {
		Name string `json:"name"`
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

func uploadSourceCode(w http.ResponseWriter, r *http.Request) {
	if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "multipart/form-data") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		fmt.Printf("user has not uploaded multipart form data, got: %s\n", contentType)
		return
	}

	language := strings.ToUpper(strings.TrimSpace(r.FormValue("sourceCodeLanguage")))
	if len(language) == 0 {
		fmt.Println("user did not specify the source code language")
		return
	}

	if !slices.Contains(supportedSourceCodeLanguages, language) {
		fmt.Printf("source code language %s is not supported\n", language)
		return
	}

	fmt.Printf("user has uploaded source code of type %s\n", language)

	file, header, err := r.FormFile("sourceCode")
	if err != nil {
		fmt.Printf("no source code found: %v\n", err)
		return
	}

	if header.Size > maxSourceCodeSize {
		fmt.Printf("source code package is too big, max size: %dMB\n", maxSourceCodeSize/megabyte)
		return
	}

	parts := strings.Split(header.Filename, ".")
	if len(parts) == 0 {
		fmt.Println("invalid file - no extension found")
		return
	}

	extension := strings.ToLower(parts[len(parts)-1])
	if extension != sourceCodeFileExtension {
		fmt.Printf("invalid file extension, expected .%s got .%s\n", sourceCodeFileExtension, extension)
		return
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
		fmt.Printf("failed to read file: %v", err)
		return
	}

	// TODO: generate custom files names and handle conflict resolution gracefully - right now we simply override.
	if err = os.WriteFile(fmt.Sprintf("%s/%s", sourceCodeDestination, header.Filename), buffer[:header.Size], 0200); err != nil {
		fmt.Printf("could not create file for source code: %v\n", err)
		return
	}

	fmt.Printf("file %s uploaded successfully\n", header.Filename)
}

func deleteSourceCode(w http.ResponseWriter, r *http.Request) {
	if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		fmt.Printf("user has not uploaded json data, got: %s\n", contentType)
		return
	}

	payload := deleteSourceCodePayload{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Printf("could not decode payload: %v\n", err)
		return
	}

	fileName := payload.Name

	if len(fileName) == 0 {
		fmt.Println("no file name has been specified")
		return
	}

	if !strings.HasSuffix(fileName, ".zip") {
		fileName += ".zip"
	}

	if err := os.Remove(fmt.Sprintf("%s/%s", sourceCodeDestination, fileName)); err != nil {
		fmt.Printf("could not delete file: %v\n", err)
		return
	}

	fmt.Printf("deleted file %s\n", fileName)
}

func main() {
	// Source code will be place here
	if err := os.MkdirAll(sourceCodeDestination, 0600); err != nil {
		panic(fmt.Sprintf("could not create source code directory: %v", err))
	}

	http.HandleFunc("POST /api/v1/sourcecode", uploadSourceCode)
	http.HandleFunc("DELETE /api/v1/sourcecode", deleteSourceCode)

	if err := http.ListenAndServe(":8080", nil); err != http.ErrServerClosed {
		panic(fmt.Sprintf("server closed unexpectedly: %v", err))
	}
}
