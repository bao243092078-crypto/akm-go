package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BudgetConfig defines per-provider request limits.
type BudgetConfig struct {
	DailyLimit   int64 `json:"daily_limit"`   // 0 = unlimited
	MonthlyLimit int64 `json:"monthly_limit"` // 0 = unlimited
}

// providerCounter tracks request counts for a single provider.
type providerCounter struct {
	DailyCount   int64  `json:"daily_count"`
	MonthlyCount int64  `json:"monthly_count"`
	DailyDate    string `json:"daily_date"`   // "2006-01-02"
	MonthlyDate  string `json:"monthly_date"` // "2006-01"
}

// budgetData is the persistent file format.
type budgetData struct {
	Config   map[string]*BudgetConfig   `json:"config"`
	Counters map[string]*providerCounter `json:"counters"`
}

// BudgetTracker manages per-provider request budgets with persistence.
type BudgetTracker struct {
	mu       sync.RWMutex
	config   map[string]*BudgetConfig
	counters map[string]*providerCounter
	file     string
}

var (
	budgetInstance *BudgetTracker
	budgetOnce     sync.Once
)

// GetBudgetTracker returns the singleton BudgetTracker.
func GetBudgetTracker() (*BudgetTracker, error) {
	var initErr error
	budgetOnce.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			initErr = err
			return
		}
		dataDir := filepath.Join(homeDir, ".apikey-manager", "data")
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			initErr = err
			return
		}
		budgetInstance, initErr = newBudgetTracker(filepath.Join(dataDir, "budget.json"))
	})
	if initErr != nil {
		return nil, initErr
	}
	return budgetInstance, nil
}

func newBudgetTracker(file string) (*BudgetTracker, error) {
	bt := &BudgetTracker{
		config:   make(map[string]*BudgetConfig),
		counters: make(map[string]*providerCounter),
		file:     file,
	}
	if err := bt.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load budget data: %w", err)
	}
	return bt, nil
}

func (bt *BudgetTracker) load() error {
	data, err := os.ReadFile(bt.file)
	if err != nil {
		return err
	}
	var bd budgetData
	if err := json.Unmarshal(data, &bd); err != nil {
		return err
	}
	if bd.Config != nil {
		bt.config = bd.Config
	}
	if bd.Counters != nil {
		bt.counters = bd.Counters
	}
	return nil
}

func (bt *BudgetTracker) save() error {
	bd := budgetData{
		Config:   bt.config,
		Counters: bt.counters,
	}
	data, err := json.MarshalIndent(bd, "", "  ")
	if err != nil {
		return err
	}
	tempFile := bt.file + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tempFile, bt.file)
}

// Check returns an error if the provider has exceeded its budget.
func (bt *BudgetTracker) Check(provider string) error {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	cfg := bt.config[provider]
	if cfg == nil {
		return nil // no limits configured
	}

	counter := bt.getOrResetCounter(provider)

	today := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")

	if cfg.DailyLimit > 0 && counter.DailyDate == today && counter.DailyCount >= cfg.DailyLimit {
		return fmt.Errorf("provider '%s' daily limit exceeded (%d/%d)", provider, counter.DailyCount, cfg.DailyLimit)
	}
	if cfg.MonthlyLimit > 0 && counter.MonthlyDate == month && counter.MonthlyCount >= cfg.MonthlyLimit {
		return fmt.Errorf("provider '%s' monthly limit exceeded (%d/%d)", provider, counter.MonthlyCount, cfg.MonthlyLimit)
	}
	return nil
}

// Record records one request for the provider. Saves asynchronously.
func (bt *BudgetTracker) Record(provider string) {
	bt.mu.Lock()
	counter := bt.ensureCounter(provider)

	today := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")

	// Reset daily counter if date changed
	if counter.DailyDate != today {
		counter.DailyCount = 0
		counter.DailyDate = today
	}
	// Reset monthly counter if month changed
	if counter.MonthlyDate != month {
		counter.MonthlyCount = 0
		counter.MonthlyDate = month
	}

	counter.DailyCount++
	counter.MonthlyCount++

	bt.mu.Unlock()

	// Async save (best-effort)
	go func() {
		bt.mu.RLock()
		defer bt.mu.RUnlock()
		_ = bt.save()
	}()
}

// SetConfig sets budget limits for a provider.
func (bt *BudgetTracker) SetConfig(provider string, daily, monthly int64) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	bt.config[provider] = &BudgetConfig{
		DailyLimit:   daily,
		MonthlyLimit: monthly,
	}
	return bt.save()
}

// ResetCounter resets the counter for a provider.
func (bt *BudgetTracker) ResetCounter(provider string) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	delete(bt.counters, provider)
	return bt.save()
}

// ProviderStats holds usage stats for display.
type ProviderStats struct {
	Provider     string
	DailyCount   int64
	DailyLimit   int64
	MonthlyCount int64
	MonthlyLimit int64
}

// GetAllStats returns stats for all configured providers.
func (bt *BudgetTracker) GetAllStats() []ProviderStats {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	// Collect all known providers
	providers := make(map[string]bool)
	for p := range bt.config {
		providers[p] = true
	}
	for p := range bt.counters {
		providers[p] = true
	}

	today := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")

	var stats []ProviderStats
	for p := range providers {
		s := ProviderStats{Provider: p}
		if cfg := bt.config[p]; cfg != nil {
			s.DailyLimit = cfg.DailyLimit
			s.MonthlyLimit = cfg.MonthlyLimit
		}
		if c := bt.counters[p]; c != nil {
			if c.DailyDate == today {
				s.DailyCount = c.DailyCount
			}
			if c.MonthlyDate == month {
				s.MonthlyCount = c.MonthlyCount
			}
		}
		stats = append(stats, s)
	}
	return stats
}

func (bt *BudgetTracker) getOrResetCounter(provider string) *providerCounter {
	c := bt.counters[provider]
	if c == nil {
		return &providerCounter{}
	}
	return c
}

func (bt *BudgetTracker) ensureCounter(provider string) *providerCounter {
	c := bt.counters[provider]
	if c == nil {
		c = &providerCounter{}
		bt.counters[provider] = c
	}
	return c
}
