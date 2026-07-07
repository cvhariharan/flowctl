package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

type ObjectInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Storage interface {
	Put(ctx context.Context, path string, reader io.Reader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	Delete(ctx context.Context, path string) error
}

type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalStorage{baseDir: baseDir}, nil
}

func (l *LocalStorage) Put(ctx context.Context, path string, reader io.Reader) error {
	fullPath := filepath.Join(l.baseDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (l *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.baseDir, path)
	return os.Open(fullPath)
}

func (l *LocalStorage) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	searchDir := filepath.Join(l.baseDir, prefix)
	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			relPath, err := filepath.Rel(l.baseDir, path)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			objects = append(objects, ObjectInfo{
				Path:       relPath,
				Name:       d.Name(),
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
		}
		return nil
	})
	return objects, err
}

func (l *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(l.baseDir, path)
	return os.RemoveAll(fullPath)
}
