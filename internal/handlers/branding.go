package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
		return echo.NewHTTPError(http.StatusNotFound)
	}

	name := c.Param("file")
	if name != filepath.Base(name) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if _, ok := brandingAssetExtensions[strings.ToLower(filepath.Ext(name))]; !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	f, err := h.brandingRoot.Open(name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	http.ServeContent(c.Response(), c.Request(), name, info.ModTime(), f)
	return nil
}

func openBrandingRoot(dir string) (*os.Root, error) {
	if dir == "" {
		return nil, nil
	}
	return os.OpenRoot(dir)
}
