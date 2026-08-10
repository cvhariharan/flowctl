package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// brandingAssetExtensions is the allowlist of file types served from the branding directory
var brandingAssetExtensions = map[string]struct{}{
	".css":  {},
	".svg":  {},
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".gif":  {},
	".webp": {},
	".ico":  {},
}

func (h *Handler) HandleGetBrandingAsset(c echo.Context) error {
	if h.brandingRoot == nil {
		return wrapError(ErrResourceNotFound, "branding asset not found", fmt.Errorf("branding directory is not configured"), nil)
	}

	name := c.Param("file")
	if name != filepath.Base(name) {
		return wrapError(ErrResourceNotFound, "branding asset not found", fmt.Errorf("branding asset name is not a bare file name: %s", name), nil)
	}
	if _, ok := brandingAssetExtensions[strings.ToLower(filepath.Ext(name))]; !ok {
		return wrapError(ErrResourceNotFound, "branding asset not found", fmt.Errorf("branding asset type is not allowed: %s", name), nil)
	}

	f, err := h.brandingRoot.Open(name)
	if err != nil {
		return wrapError(ErrResourceNotFound, "branding asset not found", err, nil)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return wrapError(ErrResourceNotFound, "branding asset not found", err, nil)
	}
	if info.IsDir() {
		return wrapError(ErrResourceNotFound, "branding asset not found", fmt.Errorf("branding asset is a directory: %s", name), nil)
	}

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return wrapError(ErrOperationFailed, "could not read branding asset", err, nil)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return wrapError(ErrOperationFailed, "could not read branding asset", err, nil)
	}

	c.Response().Header().Set("ETag", fmt.Sprintf(`"%x"`, sum.Sum(nil)))
	c.Response().Header().Set("Cache-Control", "no-cache")

	http.ServeContent(c.Response(), c.Request(), name, time.Time{}, f)
	return nil
}

func openBrandingRoot(dir string) (*os.Root, error) {
	if dir == "" {
		return nil, nil
	}
	return os.OpenRoot(dir)
}
