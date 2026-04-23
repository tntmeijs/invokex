package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/tntmeijs/invokex/src/control/application"
	"github.com/tntmeijs/invokex/src/control/config"
	"github.com/tntmeijs/invokex/src/control/firecracker"
	"github.com/tntmeijs/invokex/src/control/server"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
)

type (
	uploadApplicationResponseBody struct {
		Id string `json:"id"`
	}

	applicationFileUploadEvent struct {
		Path string `json:"path"`
	}

	messageResponseBody struct {
		Message string `json:"message"`
	}

	controlPlane struct {
		server                         server.HttpServer
		manager                        firecracker.FirecrackerManager
		config                         config.Config
		applicationFileUploadPublisher rabbitmq.Publisher // TODO: this should live somewhere else
	}
)

const (
	runtimeField         string = "runtime"
	applicationFileField string = "application"

	queueNameApplicationFileUpload string = "application-file-uploaded"

	applicationFileExtension string = "zip"

	kilobyte           int64 = 1024
	megabyte           int64 = kilobyte * kilobyte
	maxApplicationSize int64 = 50 * megabyte
)

var supportedRuntimes = []firecracker.Runtime{
	"golang",
}

func (c *controlPlane) uploadApplication(r server.Request) (server.Response, error) {
	if contentType := r.Raw.Header.Get("Content-Type"); !strings.Contains(contentType, "multipart/form-data") {
		if len(contentType) == 0 {
			contentType = "unknown"
		}

		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("user has not uploaded multipart form data, got: %s", contentType)})
	}

	runtime := firecracker.NewRuntime(strings.TrimSpace(r.Raw.FormValue(runtimeField)))
	if len(runtime) == 0 {
		fmt.Println("user did not specify the runtime")

		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: "user did not specify the runtime"})
	}

	if !slices.Contains(supportedRuntimes, runtime) {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("runtime %s is not supported", runtime)})
	}

	file, header, err := r.Raw.FormFile(applicationFileField)
	if err != nil {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("no application found: %v", err)})
	}

	if header.Size > maxApplicationSize {
		return server.ReturnResponse(http.StatusRequestEntityTooLarge, messageResponseBody{Message: fmt.Sprintf("application package is too big, max size: %dMB", maxApplicationSize/megabyte)})
	}

	parts := strings.Split(header.Filename, ".")
	if len(parts) == 0 {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: "invalid file - no extension found"})
	}

	extension := strings.ToLower(parts[len(parts)-1])
	if extension != applicationFileExtension {
		return server.ReturnResponse(http.StatusBadRequest, messageResponseBody{Message: fmt.Sprintf("invalid file extension, expected .%s got .%s", applicationFileExtension, extension)})
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
		return server.ReturnError(fmt.Errorf("failed to read file: %w", err))
	}

	applicationId := application.NewApplicationId()
	outFile := path.Join(c.config.Application.Upload.Directory, applicationId.String())
	if err = os.WriteFile(outFile, buffer[:header.Size], 0644); err != nil {
		return server.ReturnError(fmt.Errorf("could not create file for application: %w", err))
	}

	fmt.Printf("user has uploaded application %s for runtime %s\n", applicationId, runtime)

	if err = c.applicationFileUploadPublisher.SendJson(r.Raw.Context(), applicationFileUploadEvent{Path: outFile}); err != nil {
		return server.ReturnError(fmt.Errorf("failed to send file upload event: %w", err))
	}

	return server.ReturnResponse(http.StatusOK, uploadApplicationResponseBody{Id: applicationId.String()})
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
	if err := config.CreateDirectories(); err != nil {
		panic(fmt.Sprintf("could not create one or multiple directories specified in the configuration file: %w", err))
	}

	firecrackerManager := firecracker.NewManager(firecracker.FirecrackerConfig{
		FirecrackerPath:     config.Firecracker.Instance.Path,
		KernelImagePath:     config.Firecracker.Kernel.Path,
		KernelRootFsPath:    config.Firecracker.RootFilesystem.Path,
		LogDirectory:        config.Firecracker.Directories.FirecrackerLogs,
		VmConfigDirectory:   config.Firecracker.Directories.VmConfigurations,
		ApiSocketsDirectory: config.Firecracker.Directories.ApiSockets,
		VmLogsDirectory:     config.Firecracker.Directories.VmLogs,
	})

	firecrackerManager.RegisterVmConfig(firecracker.NewGolangConfig(firecracker.LogLevelDebug))
	firecrackerManager.RegisterVmConfig(firecracker.NewNodeConfig(25, firecracker.LogLevelDebug))

	mainCtx := context.Background()
	rabbitmqInstance := rabbitmq.NewInstance(mainCtx, config.MessageBroker.Username, config.MessageBroker.Password, config.MessageBroker.Host)
	defer rabbitmqInstance.Close(mainCtx)

	rabbitmqConnection, err := rabbitmqInstance.Connect(mainCtx, rabbitmq.WithClassicQueue(queueNameApplicationFileUpload))
	if err != nil {
		panic(fmt.Sprintf("could not establish a connection with rabbitmq: %s", err.Error()))
	}

	applicationFileUploadConsumer, err := rabbitmqConnection.NewConsumer(mainCtx, queueNameApplicationFileUpload)
	if err != nil {
		panic(fmt.Sprintf("could not create application file upload consumer: %s", err.Error()))
	}

	applicationFileUploadPublisher, err := rabbitmqConnection.NewQueuePublisher(mainCtx, queueNameApplicationFileUpload)
	if err != nil {
		panic(fmt.Sprintf("could not create application file upload publisher: %s", err.Error()))
	}

	ctrl := controlPlane{
		manager:                        firecrackerManager,
		server:                         *server.NewHttpServer(),
		config:                         config,
		applicationFileUploadPublisher: applicationFileUploadPublisher,
	}

	applicationFileUploadConsumer.Listen(mainCtx, ctrl.onFileUpload)
	defer applicationFileUploadConsumer.Stop()

	err = ctrl.server.
		RegisterRoute(server.HttpPost, "/api/v1/application", ctrl.uploadApplication).
		RegisterRoute(server.HttpPost, "/api/v1/vm", ctrl.invokeVm).
		RegisterRoute(server.HttpDelete, "/api/v1/vm/{id}", ctrl.deleteVm).
		Run(":8080")

	if err != nil {
		panic(fmt.Sprintf("server has closed: %s", err.Error()))
	}
}

func (c *controlPlane) onFileUpload(msg rabbitmq.Message, err error) rabbitmq.MessageOutcome {
	var event applicationFileUploadEvent
	if err := msg.AsJson(&event); err != nil {
		fmt.Printf("failed to consume file upload message: %v", err)
		return rabbitmq.MessageOutcomeRequeue
	}

	processor := application.NewFileUploadProcessor()
	if err := processor.UnpackArchive(event.Path, c.config.Application.Upload.Output); err != nil {
		fmt.Printf("failed to unpack archive: %v", err)
		return rabbitmq.MessageOutcomeRequeue
	}

	fmt.Printf("processed file upload successfully: %s\n", event.Path)
	return rabbitmq.MessageOutcomeAccept
}
