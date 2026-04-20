package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/tntmeijs/invokex/control/config"
	"github.com/tntmeijs/invokex/control/firecracker"
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

	controlPlane struct {
		server  server.HttpServer
		manager firecracker.FirecrackerManager
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

func (c *controlPlane) uploadSourceCode(r server.Request) (server.Response, error) {
	if contentType := r.Raw.Header.Get("Content-Type"); !strings.Contains(contentType, "multipart/form-data") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("user has not uploaded multipart form data, got: %s", contentType)})
	}

	language := strings.ToUpper(strings.TrimSpace(r.Raw.FormValue("sourceCodeLanguage")))
	if len(language) == 0 {
		fmt.Println("user did not specify the source code language")

		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: "user did not specify the source code language"})
	}

	if !slices.Contains(supportedSourceCodeLanguages, language) {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("source code language %s is not supported", language)})
	}

	fmt.Printf("user has uploaded source code of type %s\n", language)

	file, header, err := r.Raw.FormFile("sourceCode")
	if err != nil {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("no source code found: %v", err)})
	}

	if header.Size > maxSourceCodeSize {
		return server.ReturnResponse(http.StatusRequestEntityTooLarge, messageResponseBody{Message: fmt.Sprintf("source code package is too big, max size: %dMB", maxSourceCodeSize/megabyte)})
	}

	parts := strings.Split(header.Filename, ".")
	if len(parts) == 0 {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: "invalid file - no extension found"})
	}

	extension := strings.ToLower(parts[len(parts)-1])
	if extension != sourceCodeFileExtension {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("invalid file extension, expected .%s got .%s", sourceCodeFileExtension, extension)})
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
		return server.ReturnError(fmt.Errorf("failed to read file: %v", err))
	}

	// TODO: generate custom files names and handle conflict resolution gracefully - right now we simply override.
	if err = os.WriteFile(fmt.Sprintf("%s/%s", sourceCodeDestination, header.Filename), buffer[:header.Size], 0200); err != nil {
		return server.ReturnError(fmt.Errorf("could not create file for source code: %v", err))
	}

	return server.ReturnResponse(http.StatusOK, messageResponseBody{Message: fmt.Sprintf("file %s uploaded successfully", header.Filename)})
}

func (c *controlPlane) deleteSourceCode(r server.Request) (server.Response, error) {
	if contentType := r.Raw.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("user has not uploaded json data, got: %s", contentType)})
	}

	payload := deleteSourceCodePayload{}
	if err := json.NewDecoder(r.Raw.Body).Decode(&payload); err != nil {
		return server.ReturnError(fmt.Errorf("could not decode payload: %v", err))
	}

	fileName := payload.Name

	if len(fileName) == 0 {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: "no file name has been specified"})
	}

	if !strings.HasSuffix(fileName, ".zip") {
		fileName += ".zip"
	}

	if err := os.Remove(fmt.Sprintf("%s/%s", sourceCodeDestination, fileName)); err != nil {
		return server.ReturnResponse(http.StatusNotFound, messageResponseBody{Message: fmt.Sprintf("no source code with name %s was found", fileName)})
	}

	fmt.Printf("deleted file %s\n", fileName)
	return server.ReturnResponse(http.StatusNoContent)
}

func (c *controlPlane) invokeVm(r server.Request) (server.Response, error) {
	vmId, err := c.manager.InstantiateVm(firecracker.NewRuntime("golang"))
	if err != nil {
		return server.ReturnError(err)
	}

	return server.ReturnResponse(
		http.StatusOK,
		struct {
			VmId string `json:"vmId"`
		}{VmId: vmId},
	)
}

func (c *controlPlane) deleteVm(r server.Request) (server.Response, error) {
	vmId := strings.TrimSpace(r.Raw.PathValue("id"))
	if len(vmId) == 0 {
		return server.ReturnResponse(http.StatusBadRequest)
	}

	if err := c.manager.KillVm(vmId); err != nil {
		return server.ReturnError(err)
	}

	return server.ReturnResponse(http.StatusOK)
}

func main() {
	// TODO: wrap application with a signal listener so we can clean up any pending VMs when we receive SIGTERM.
	config := config.MustLoadFromArgs()

	// Source code will be place here
	if err := os.MkdirAll(sourceCodeDestination, 0600); err != nil {
		panic(fmt.Sprintf("could not create source code directory: %v", err))
	}

	firecrackerManager := firecracker.NewManager(firecracker.FirecrackerConfig{
		FirecrackerPath:     config.Firecracker.Instance.Path,
		KernelImagePath:     config.Firecracker.Kernel.Path,
		KernelRootFsPath:    config.Firecracker.RootFilesystem.Path,
		LogDirectory:        config.Firecracker.Instance.LogDirectory,
		VmConfigDirectory:   config.Firecracker.Instance.VmConfigDirectory,
		ApiSocketsDirectory: config.Firecracker.Instance.ApiSocketsDirectory,
	})

	firecrackerManager.RegisterVmConfig(firecracker.NewGolangConfig(firecracker.LogLevelDebug))
	firecrackerManager.RegisterVmConfig(firecracker.NewNodeConfig(25, firecracker.LogLevelDebug))

	ctrl := controlPlane{
		manager: firecrackerManager,
		server:  *server.NewHttpServer(),
	}

	err := ctrl.server.
		RegisterRoute(server.HttpPost, "/api/v1/sourcecode", ctrl.uploadSourceCode).
		RegisterRoute(server.HttpDelete, "/api/v1/sourcecode", ctrl.deleteSourceCode).
		RegisterRoute(server.HttpPost, "/api/v1/vm", ctrl.invokeVm).
		RegisterRoute(server.HttpDelete, "/api/v1/vm/{id}", ctrl.deleteVm).
		Run(":8080")

	if err != nil {
		panic(fmt.Sprintf("server has closed: %s", err.Error()))
	}
}
