package router

import (
	"net/http"
	"os"
	"path/filepath"

	"image-processor/internal/http-server/handler/image"
	"image-processor/internal/http-server/middleware"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	ImageHandler *image.ImageHandler
}

func SetupRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RecoveryMiddleware)
	r.Use(middleware.LoggingMiddleware)

	workDir, _ := os.Getwd()
	staticDir := http.Dir(filepath.Join(workDir, "web", "static"))
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(staticDir)))

	r.Route("/api", func(r chi.Router) {
		r.Route("/images", func(r chi.Router) {
			// r.Post("/upload", h.ImageHandler.UploadImage)
			// r.Get("/{id}", h.ImageHandler.GetImage)
			// r.Get("/{id}/status", h.ImageHandler.GetStatus)
			// r.Delete("/{id}", h.ImageHandler.DeleteImage)
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(workDir, "web", "index.html")
		http.ServeFile(w, r, indexPath)
	})

	return r
}
