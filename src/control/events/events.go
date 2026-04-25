package events

type (
	UnpackArchiveEvent struct {
		Path string `json:"path"`
	}

	CreateFilesystemEvent struct {
		Type           string `json:"type"`
		FileSystemRoot string `json:"root"`
	}
)
