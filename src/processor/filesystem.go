package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tntmeijs/invokex/src/control/events"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
)

const filesystemTypeExt4 string = "ext4"

type FilesystemBuilder struct {
	outputDirectory string
}

func NewFilesystemBuilder(outputDirectory string) FilesystemBuilder {
	return FilesystemBuilder{outputDirectory: outputDirectory}
}

func (f FilesystemBuilder) onEvent(ctx context.Context, message rabbitmq.Message) rabbitmq.MessageOutcome {
	var event events.CreateFilesystemEvent
	if err := message.AsJson(&event); err != nil {
		fmt.Printf("failed to parse create filesystem event: %v\n", err)
		return rabbitmq.MessageOutcomeDiscard
	}

	filesystemType := strings.ToLower(event.Type)

	if filesystemType != filesystemTypeExt4 {
		fmt.Printf("discarding create filesystem event because the requested filesystem is not supported: %s\n", filesystemType)
		return rabbitmq.MessageOutcomeDiscard
	}

	if info, err := os.Stat(event.FileSystemRoot); err == nil && info.IsDir() {
		// Filesystem root folder exists - time to try to convert this into an EXT4 filesystem.
		if err = f.buildExt4(); err != nil {
			fmt.Printf("could not build ext4 filesystem: %v\n", err)
			return rabbitmq.MessageOutcomeDiscard
		}

		// All good - EXT4 filesystem now exists. :)
		return rabbitmq.MessageOutcomeAccept
	}

	fmt.Printf("failed to create filesystem because the specified filesystem root does not exist: %s\n", event.FileSystemRoot)
	return rabbitmq.MessageOutcomeDiscard
}

func (f FilesystemBuilder) buildExt4() error {
	fmt.Printf("TODO: create ext4 filesystem\n")
	return nil
}
