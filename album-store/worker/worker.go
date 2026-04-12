package worker

import (
	"context"
	"errors"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"album-store/store"
)

// Job carries everything the worker needs to upload a photo and update its status.
type Job struct {
	PhotoID     string
	AlbumID     string
	Data        []byte
	Key         string
	ContentType string
}

// Pool is a fixed-size goroutine pool that processes photo upload jobs.
type Pool struct {
	jobs    chan Job
	db      *store.DynamoDB
	s3      *store.S3Store
	workers int
}

// NewPool creates a pool.  bufferSize is the job queue depth.
func NewPool(workers, bufferSize int, db *store.DynamoDB, s3 *store.S3Store) *Pool {
	return &Pool{
		jobs:    make(chan Job, bufferSize),
		db:      db,
		s3:      s3,
		workers: workers,
	}
}

// Start launches worker goroutines.  Call once at startup.
func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		go p.run()
	}
	log.Printf("worker pool started: %d goroutines, queue depth %d", p.workers, cap(p.jobs))
}

// Submit enqueues a job.  Blocks only if the queue is full.
func (p *Pool) Submit(job Job) {
	p.jobs <- job
}

func (p *Pool) run() {
	for job := range p.jobs {
		ctx := context.Background()

		url, err := p.s3.Upload(ctx, job.Key, job.Data, job.ContentType)
		if err != nil {
			log.Printf("ERROR worker: S3 upload photo=%s: %v", job.PhotoID, err)
			if err2 := p.db.UpdatePhotoStatus(ctx, job.AlbumID, job.PhotoID, "failed", ""); err2 != nil {
				// Ignore conditional check failures (photo was deleted/completed)
				var ccf *types.ConditionalCheckFailedException
				if !errors.As(err2, &ccf) {
					log.Printf("ERROR worker: mark failed photo=%s: %v", job.PhotoID, err2)
				}
			}
			continue
		}

		if err := p.db.UpdatePhotoStatus(ctx, job.AlbumID, job.PhotoID, "completed", url); err != nil {
			// Ignore conditional check failures (photo was deleted while processing)
			var ccf *types.ConditionalCheckFailedException
			if errors.As(err, &ccf) {
				log.Printf("DEBUG worker: photo=%s was deleted or already completed, skipping update", job.PhotoID)
			} else {
				log.Printf("ERROR worker: mark completed photo=%s: %v", job.PhotoID, err)
			}
		}
	}
}