package config

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
)

type catwalkClient interface {
	GetProviders(context.Context, string) ([]catwalk.Provider, error)
}

var _ syncer[[]catwalk.Provider] = (*catwalkSync)(nil)

type catwalkSync struct {
	once       sync.Once
	result     []catwalk.Provider
	cache      cache[[]catwalk.Provider]
	client     catwalkClient
	autoupdate bool
	init       atomic.Bool
}

func (s *catwalkSync) Init(client catwalkClient, path string, autoupdate bool) {
	s.client = client
	s.cache = newCache[[]catwalk.Provider](path)
	s.autoupdate = autoupdate
	s.init.Store(true)
}

func (s *catwalkSync) Get(ctx context.Context) ([]catwalk.Provider, error) {
	if !s.init.Load() {
		panic("called Get before Init")
	}

	var throwErr error
	s.once.Do(func() {
		if !s.autoupdate {
			slog.Info("Using embedded Catwalk providers")
			s.result = embedded.GetAll()
			patchReasoningLevels(s.result)
			return
		}

		cached, etag, cachedErr := s.cache.Get()
		if len(cached) == 0 || cachedErr != nil {
			// if cached file is empty, default to embedded providers
			cached = embedded.GetAll()
		}

		slog.Info("Fetching providers from Catwalk")
		result, err := s.client.GetProviders(ctx, etag)
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("Catwalk providers not updated in time")
			s.result = cached
			patchReasoningLevels(s.result)
			return
		}
		if errors.Is(err, catwalk.ErrNotModified) {
			slog.Info("Catwalk providers not modified")
			s.result = cached
			patchReasoningLevels(s.result)
			return
		}
		if err != nil {
			// On error, fall back to cached (which defaults to embedded if empty).
			s.result = cached
			patchReasoningLevels(s.result)
			return
		}
		if len(result) == 0 {
			s.result = cached
			throwErr = errors.New("empty providers list from catwalk")
			patchReasoningLevels(s.result)
			return
		}

		s.result = result
		throwErr = s.cache.Store(result)
		patchReasoningLevels(s.result)
	})
	return s.result, throwErr
}

// patchReasoningLevels supplements missing reasoning levels in the embedded
// catwalk data. Opus 4.7 supports "xhigh" but the embedded config may not
// include it.
func patchReasoningLevels(providers []catwalk.Provider) {
	for i := range providers {
		for j := range providers[i].Models {
			m := &providers[i].Models[j]
			if m.ID == "claude-opus-4-7" && len(m.ReasoningLevels) > 0 && !slices.Contains(m.ReasoningLevels, "xhigh") {
				m.ReasoningLevels = []string{"low", "medium", "high", "xhigh", "max"}
			}
		}
	}
}
