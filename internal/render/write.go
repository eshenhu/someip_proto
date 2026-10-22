package render

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path atomically with contents. The temporary file
// is created in path's directory so the final rename remains on one filesystem.
func WriteFileAtomic(path string, contents []byte, mode fs.FileMode) (err error) {
	if path == "" {
		return fmt.Errorf("write artifact: output path is empty")
	}
	if mode == 0 {
		mode = 0o644
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary artifact in %q: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			closeErr := temporary.Close()
			closed = true
			if err == nil && closeErr != nil {
				err = fmt.Errorf("close temporary artifact: %w", closeErr)
			}
		}
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary artifact permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		return fmt.Errorf("write temporary artifact: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary artifact: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close temporary artifact: %w", err)
	}
	closed = true
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace artifact %q: %w", path, err)
	}
	return nil
}
