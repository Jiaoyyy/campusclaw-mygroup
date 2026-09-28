package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"campusclaw/backend/internal/auth"
	"campusclaw/backend/internal/config"
	"campusclaw/backend/internal/db"
	"campusclaw/backend/internal/materials"
	"campusclaw/backend/internal/retrieval"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	database, err := db.Open(cfg)
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.WaitReady(ctx, database); err != nil {
		log.Fatal("database unavailable")
	}
	if err := db.Migrate(ctx, database); err != nil {
		log.Fatal("database migration failed")
	}
	if err := db.Seed(ctx, database, cfg); err != nil {
		log.Fatal("database seed failed")
	}
	if err := materials.CleanupOrphans(ctx, database, cfg.UploadDir, time.Hour); err != nil {
		log.Fatal("upload cleanup failed")
	}
	authService, err := auth.New(database, cfg)
	if err != nil {
		log.Fatal("authentication initialization failed")
	}
	mux := http.NewServeMux()
	embedder := retrieval.NewEmbedder(cfg)
	vectors := retrieval.NewVectorStore(cfg)
	indexer := retrieval.NewIndexer(database, embedder, vectors, cfg.VectorEnabled())
	mux.HandleFunc("/api/login", authService.Login)
	mux.HandleFunc("/api/logout", authService.Logout)
	mux.HandleFunc("/api/me", authService.Me)
	materials.New(database, cfg, authService).WithIndexer(indexer).Register(mux)
	retrieval.New(database, authService, embedder, vectors, retrieval.NewAnswerer(cfg), cfg.VectorEnabled(), cfg.AnswerEnabled()).Register(mux)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		auth.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Printf("listening on %s", cfg.ListenAddr)
	go func() {
		for {
			indexCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			failed, err := indexer.Backfill(indexCtx)
			cancel()
			if err != nil || failed > 0 {
				log.Printf("index backfill: %d materials need retry", failed)
			}
			if !cfg.VectorEnabled() {
				return // Restart with gateway configuration to retry failed chunks.
			}
			time.Sleep(2 * time.Minute)
		}
	}()
	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(server.ListenAndServe())
}
