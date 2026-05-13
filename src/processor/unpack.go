package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/tntmeijs/invokex/src/control/events"
	"github.com/tntmeijs/invokex/src/pubsub/rabbitmq"
)

type ArchiveUnpacker struct {
	outputDirectory     string
	filesystemPublisher rabbitmq.Publisher
}

func NewArchiveUnpacker(outputDirectory string, filesystemPublisher rabbitmq.Publisher) ArchiveUnpacker {
	return ArchiveUnpacker{outputDirectory: outputDirectory, filesystemPublisher: filesystemPublisher}
}

func (u ArchiveUnpacker) onEvent(ctx context.Context, message rabbitmq.Message) rabbitmq.MessageOutcome {
	var unpackArchiveEvent events.UnpackArchiveEvent
	if err := message.AsJson(&unpackArchiveEvent); err != nil {
		fmt.Printf("failed to parse unpack archive event: %v\n", err)
		return rabbitmq.MessageOutcomeDiscard
	}

	if err := u.unpackArchive(unpackArchiveEvent.Path); err != nil {
		fmt.Printf("failed to unpack archive: %v\n", err)
		return rabbitmq.MessageOutcomeDiscard
	}

	inputFileId := filepath.Base(unpackArchiveEvent.Path)
	outputDirectory := path.Join(u.outputDirectory, inputFileId)
	createFilesystemEvent := events.CreateFilesystemEvent{Type: "ext4", FileSystemRoot: outputDirectory}

	if err := u.filesystemPublisher.SendJson(ctx, createFilesystemEvent); err != nil {
		fmt.Printf("unpacking was successful but could not publish create filesystem event: %v\n", err.Error())
		return rabbitmq.MessageOutcomeDiscard
	}

	return rabbitmq.MessageOutcomeAccept
}

func (u ArchiveUnpacker) unpackArchive(archivePath string) error {
	inputFileId := filepath.Base(archivePath)
	outputDirectory := path.Join(u.outputDirectory, inputFileId)
	if err := os.MkdirAll(outputDirectory, 0744); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open application archive: %w", err)
	}

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			target := path.Join(outputDirectory, file.Name)
			if err = os.MkdirAll(target, 0744); err != nil {
				return fmt.Errorf("failed to create directory in output %s: %w", target, err)
			}

			continue
		}

		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file %s from application archive %s: %w", file.Name, archivePath, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read contents of file %s in archive %s: %w", file.Name, archivePath, err)
		}

		outFileName := path.Join(outputDirectory, file.Name)
		if err = os.WriteFile(outFileName, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outFileName, err)
		}
	}

	defer zipReader.Close()
	return nil
}
