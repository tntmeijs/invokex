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

	"github.com/tntmeijs/invokex/src/configuration"
	"github.com/tntmeijs/invokex/src/control/application"
	"github.com/tntmeijs/invokex/src/control/events"
	"github.com/tntmeijs/invokex/src/control/firecracker"
	"github.com/tntmeijs/invokex/src/control/server"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
	"go.uber.org/dig"
)

type (
	uploadApplicationResponseBody struct {
		Id string `json:"id"`
	}

	messageResponseBody struct {
		Message string `json:"message"`
	}

	controlPlane struct {
		container *dig.Container
	}
)

const (
	runtimeField         string = "runtime"
	applicationFileField string = "application"

	exchangeNameUserApplication                                     string = "invokex.user.application"
	exchangeBindingKeyNewUserApplicationArchiveUnpack               string = "user.application.archive.unpack"
	exchangeBindingKeyNewUserApplicationArchiveCreateFilesystemExt4 string = "user.application.archive.filesystem.ext4"

	queueNameUnpackArchive    string = "user.application.archive.unpack"
	queueNameCreateFilesystem string = "user.application.archive.filesystem"

	applicationFileExtension string = "zip"

	kilobyte           int64 = 1024
	megabyte           int64 = kilobyte * kilobyte
	maxApplicationSize int64 = 50 * megabyte
)

var supportedRuntimes = []firecracker.Runtime{
	"golang",
}

// The newControlPlane method instantiates a new controlPlane instance.
func newControlPlane() (controlPlane, error) {
	dependencyContainer := dig.New()
	for _, f := range dependencyProviderFuncs {
		if err := dependencyContainer.Provide(f); err != nil {
			return controlPlane{}, fmt.Errorf("could not provide dependency: %w", err)
		}
	}

	return controlPlane{container: dependencyContainer}, nil
}

// MustGetDependency attempts to fetch a dependency from the dependency container in controlPlane and panics if the dependency is not available.
func MustGetDependency[T any](ctrl controlPlane) T {
	t, err := GetDependency[T](ctrl)
	if err != nil {
		panic(err)
	}

	return t
}

// GetDependency attempts to fetch a dependency from the dependency container in controlPlane.
func GetDependency[T any](ctrl controlPlane) (T, error) {
	var zeroT T

	if err := ctrl.container.Invoke(func(t T) { zeroT = t }); err != nil {
		return zeroT, fmt.Errorf("failed to fetch dependency of type %T: %w", zeroT, err)
	}

	return zeroT, nil
}

// TODO: wrap application with a signal listener so we can clean up any pending VMs when we receive SIGTERM.
func main() {
	mainCtx := context.Background()

	ctrl, err := newControlPlane()
	if err != nil {
		panic(fmt.Sprintf("could not create control plane: %s", err.Error()))
	}

	firecrackerManager := MustGetDependency[firecracker.FirecrackerManager](ctrl)
	rabbitmqInstance := MustGetDependency[rabbitmq.Instance](ctrl)
	globalConfig := MustGetDependency[configuration.Configuration](ctrl)
	fileProcessor := MustGetDependency[application.FileUploadProcessor](ctrl)

	firecrackerManager.RegisterVmConfig(firecracker.NewGolangConfig(firecracker.LogLevelDebug))
	firecrackerManager.RegisterVmConfig(firecracker.NewNodeConfig(25, firecracker.LogLevelDebug))

	defer rabbitmqInstance.Close(mainCtx)

	rabbitmqConnection, err := rabbitmqInstance.Connect(
		mainCtx,
		rabbitmq.WithClassicQueue(queueNameUnpackArchive),
		rabbitmq.WithClassicQueue(queueNameCreateFilesystem),
		rabbitmq.WithTopicExchange(exchangeNameUserApplication),
		rabbitmq.WithExchangeToQueueBinding(exchangeNameUserApplication, queueNameUnpackArchive, exchangeBindingKeyNewUserApplicationArchiveUnpack),
		rabbitmq.WithExchangeToQueueBinding(exchangeNameUserApplication, queueNameCreateFilesystem, exchangeBindingKeyNewUserApplicationArchiveCreateFilesystemExt4),
	)
	if err != nil {
		panic(fmt.Sprintf("could not establish a connection with rabbitmq: %s", err.Error()))
	}

	applicationFileUploadConsumer, err := rabbitmqConnection.NewConsumer(mainCtx, queueNameUnpackArchive)
	if err != nil {
		panic(fmt.Sprintf("could not create application file upload consumer: %s", err.Error()))
	}

	createFilesystemConsumer, err := rabbitmqConnection.NewConsumer(mainCtx, queueNameCreateFilesystem)
	if err != nil {
		panic(fmt.Sprintf("could not create filesystem consumer: %s", err.Error()))
	}

	applicationFileUploadPublisher, err := rabbitmqConnection.NewQueuePublisher(mainCtx, queueNameUnpackArchive)
	if err != nil {
		panic(fmt.Sprintf("could not create application file upload publisher: %s", err.Error()))
	}
	defer applicationFileUploadPublisher.Stop(mainCtx)

	applicationFileCreateFilesystemPublisher, err := rabbitmqConnection.NewQueuePublisher(mainCtx, queueNameCreateFilesystem)
	if err != nil {
		panic(fmt.Sprintf("could not create application ext4 filesystem publisher: %s", err.Error()))
	}
	defer applicationFileCreateFilesystemPublisher.Stop(mainCtx)

	applicationFileUploadConsumer.Listen(mainCtx, func(ctx context.Context, msg rabbitmq.Message) rabbitmq.MessageOutcome {
		return onFileUploadEvent(ctx, fileProcessor, applicationFileCreateFilesystemPublisher, globalConfig.Application.Upload.Output, msg)
	})
	defer applicationFileUploadConsumer.Stop()

	createFilesystemConsumer.Listen(mainCtx, onCreateFilesystemEvent)
	defer createFilesystemConsumer.Stop()

	err = server.NewHttpServer().
		RegisterRoute(server.HttpPost, "/api/v1/application", func(r server.Request) (server.Response, error) {
			return uploadApplication(globalConfig, applicationFileUploadPublisher, r)
		}).
		RegisterRoute(server.HttpPost, "/api/v1/vm", func(r server.Request) (server.Response, error) { return invokeVm(firecrackerManager, r) }).
		RegisterRoute(server.HttpDelete, "/api/v1/vm/{id}", func(r server.Request) (server.Response, error) { return deleteVm(firecrackerManager, r) }).
		Run(":8080")

	if err != nil {
		panic(fmt.Sprintf("server has closed: %s", err.Error()))
	}
}

func onFileUploadEvent(ctx context.Context, processor application.FileUploadProcessor, createFilesystemPublisher rabbitmq.Publisher, outputDirectory string, msg rabbitmq.Message) rabbitmq.MessageOutcome {
	var event events.UnpackArchiveEvent
	if err := msg.AsJson(&event); err != nil {
		fmt.Printf("failed to consume file upload event: %s\n", err.Error())
		return rabbitmq.MessageOutcomeDiscard
	}

	archiveRoot, err := processor.UnpackArchive(event.Path, outputDirectory)

	if err != nil {
		fmt.Printf("failed to unpack archive: %v\n", err)
		return rabbitmq.MessageOutcomeDiscard
	}

	fmt.Printf("processed file upload successfully: %s\n", event.Path)

	if err = createFilesystemPublisher.SendJson(ctx, events.CreateFilesystemEvent{Type: "ext4", FileSystemRoot: archiveRoot}); err != nil {
		fmt.Printf("unpacking was successful but could not publish create filesystem event: %s\n", err.Error())
		return rabbitmq.MessageOutcomeDiscard
	}

	return rabbitmq.MessageOutcomeAccept
}

func onCreateFilesystemEvent(ctx context.Context, msg rabbitmq.Message) rabbitmq.MessageOutcome {
	var event events.CreateFilesystemEvent
	if err := msg.AsJson(&event); err != nil {
		fmt.Printf("failed to consume create filesystem event: %s\n", err.Error())
		return rabbitmq.MessageOutcomeDiscard
	}

	fmt.Printf("received create file system event: %v\n", string(msg.Data))
	return rabbitmq.MessageOutcomeAccept
}

func uploadApplication(config configuration.Configuration, publisher rabbitmq.Publisher, r server.Request) (server.Response, error) {
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
	outFile := path.Join(config.Application.Upload.Directory, applicationId.String())
	if err = os.WriteFile(outFile, buffer[:header.Size], 0644); err != nil {
		return server.ReturnError(fmt.Errorf("could not create file for application: %w", err))
	}

	fmt.Printf("user has uploaded application %s for runtime %s\n", applicationId, runtime)

	if err = publisher.SendJson(r.Raw.Context(), events.UnpackArchiveEvent{Path: outFile}); err != nil {
		return server.ReturnError(fmt.Errorf("failed to send file upload event: %w", err))
	}

	return server.ReturnResponse(http.StatusOK, uploadApplicationResponseBody{Id: applicationId.String()})
}

func invokeVm(firecrackerManager firecracker.FirecrackerManager, _ server.Request) (server.Response, error) {
	vmId, err := firecrackerManager.InstantiateVm(firecracker.NewRuntime("golang")) // TODO: do not hardcode this
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

func deleteVm(firecrackerManager firecracker.FirecrackerManager, r server.Request) (server.Response, error) {
	vmId := strings.TrimSpace(r.Raw.PathValue("id"))
	if len(vmId) == 0 {
		return server.ReturnResponse(http.StatusBadRequest)
	}

	if err := firecrackerManager.KillVm(vmId); err != nil {
		return server.ReturnError(err)
	}

	return server.ReturnResponse(http.StatusOK)
}
