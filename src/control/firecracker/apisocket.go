package firecracker

import (
	"fmt"
	"os"
)

type apiSocket struct {
	path string
}

func newApiSocket(id VmId, directory string) apiSocket {
	path := fmt.Sprintf("%s/%s.log", directory, id)
	return apiSocket{path: path}
}

func (s apiSocket) close() error {
	return os.Remove(s.path)
}
