package output

import (
	"context"
	"time"
)

type BatchWriter struct {
	cfg      BatchWriterConfig
	Handler  BatchCallback
	buffer   []bufferItem
	len      int
	done     chan bool
	stopDone chan bool
	items    chan interface{}
}

type bufferItem struct {
	v       interface{}
	attempt int
}

type BatchCallback func(ctx context.Context, items []interface{}) []bool

type BatchWriterConfig struct {
	BatchSize  int
	MaxRetries int
	Interval   time.Duration
	Timeout    time.Duration
}

func NewBatchWriter(cfg BatchWriterConfig, cb BatchCallback) *BatchWriter {
	return &BatchWriter{
		cfg:     cfg,
		Handler: cb,
		buffer:  make([]bufferItem, cfg.BatchSize),
	}
}

func (w *BatchWriter) Start() {
	w.done = make(chan bool)
	w.items = make(chan interface{})
	w.stopDone = make(chan bool)
	ticker := time.NewTicker(w.cfg.Interval)

	go func() {
		shouldGoOn := true
		for shouldGoOn {
			select {
			case item := <-w.items:
				if w.len >= w.cfg.BatchSize {
					w.processBuffer(context.Background())
					w.len = 0
				}

				w.buffer[w.len] = bufferItem{v: item, attempt: 0}
				w.len++
			case <-w.done:
				w.processBuffer(context.Background())
				shouldGoOn = false
				w.stopDone <- true
				ticker.Stop()
			case <-ticker.C:
				w.processBuffer(context.Background())
			}
		}
	}()
}

func (w *BatchWriter) processBuffer(ctx context.Context) {
	if w.len == 0 {
		return
	}

	slice := make([]interface{}, w.len)
	for i := 0; i < w.len; i++ {
		slice[i] = w.buffer[i].v
	}

	responses := w.Handler(ctx, slice)

	var newItemsCount int
	for idx, success := range responses {
		if !success {
			item := w.buffer[idx]
			if item.attempt >= w.cfg.MaxRetries {
				continue
			}

			w.buffer[newItemsCount] = bufferItem{
				v:       item.v,
				attempt: item.attempt + 1,
			}

			newItemsCount++
		}
	}

	w.len = newItemsCount
}

func (w *BatchWriter) Stop() {
	w.done <- true
	<-w.stopDone
}

func (w *BatchWriter) Submit(items ...interface{}) {
	for _, item := range items {
		w.items <- item
	}
}
