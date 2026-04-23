package application

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// FileUploadProcessor operates on uploaded application files.
type FileUploadProcessor struct{}

// NewFileUploadProcessor creates a new FileUploadProcessor.
func NewFileUploadProcessor() FileUploadProcessor {
	return FileUploadProcessor{}
}

// UnpackArchive takes in a path to an archive and a directory in which the output should be placed.
// This method will uncompress the archive and place its uncompressed files in the output directory, preserving the original archive's structure.
func (p FileUploadProcessor) UnpackArchive(archivePath string, outputDirectory string) error {
	inputFileId := filepath.Base(archivePath)
	outputDirectory = path.Join(outputDirectory, inputFileId)
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
