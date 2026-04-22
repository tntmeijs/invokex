package application

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
)

type (
	ApplicationId string

	Service struct {
		inputDirectory  string
		outputDirectory string
	}
)

func NewApplicationId() ApplicationId {
	return ApplicationId(uuid.NewString())
}

// String returns the string representation of ApplicationId.
func (s ApplicationId) String() string {
	return string(s)
}

func NewService(inputDirectory, outputDirectory string) Service {
	return Service{
		inputDirectory:  inputDirectory,
		outputDirectory: outputDirectory,
	}
}

func (s Service) ProcessArchive(id ApplicationId) error {
	inputFile := path.Join(s.inputDirectory, id.String())
	outputDirectory := path.Join(s.outputDirectory, id.String())

	if err := os.MkdirAll(outputDirectory, 0744); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	zipReader, err := zip.OpenReader(inputFile)
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
			return fmt.Errorf("failed to open file %s from application archive %s: %w", file.Name, inputFile, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read contents of file %s in archive %s: %w", file.Name, inputFile, err)
		}

		outFileName := path.Join(outputDirectory, file.Name)
		if err = os.WriteFile(outFileName, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outFileName, err)
		}
	}

	defer zipReader.Close()
	return nil
}
