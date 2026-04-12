package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"album-store/handlers"
	"album-store/store"
	"album-store/worker"
)

func main() {
	ctx := context.Background()

	// ── AWS clients ──────────────────────────────────────────────────────────
	db, err := store.NewDynamoDB(ctx)
	if err != nil {
		log.Fatalf("FATAL init DynamoDB: %v", err)
	}

	s3, err := store.NewS3Store(ctx)
	if err != nil {
		log.Fatalf("FATAL init S3: %v", err)
	}

	// ── Worker pool ──────────────────────────────────────────────────────────
	// I/O-bound work benefits from many more goroutines than CPU cores.
	numWorkers := runtime.NumCPU() * 16
	if v := os.Getenv("WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			numWorkers = n
		}
	}
	pool := worker.NewPool(numWorkers, 4096, db, s3)
	pool.Start()

	// ── Router ───────────────────────────────────────────────────────────────
	h := handlers.New(db, s3, pool)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer) // never crash on panic
	// Omit middleware.Logger in production to reduce latency overhead.
	// Re-enable for debugging: r.Use(middleware.Logger)

	r.Get("/health", h.Health)

	r.Get("/albums", h.ListAlbums)
	r.Put("/albums/{album_id}", h.PutAlbum)
	r.Get("/albums/{album_id}", h.GetAlbum)

	r.Post("/albums/{album_id}/photos", h.UploadPhoto)
	r.Get("/albums/{album_id}/photos/{photo_id}", h.GetPhoto)
	r.Delete("/albums/{album_id}/photos/{photo_id}", h.DeletePhoto)

	// ── HTTP server ──────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
		// ReadHeaderTimeout: fast; ReadTimeout: generous for 200 MB uploads.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Minute,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("album-store listening on :%s  workers=%d", port, numWorkers)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}