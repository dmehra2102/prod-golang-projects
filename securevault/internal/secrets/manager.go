package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/dmehra2102/prod-golang-projects/securevault/pkg/logger"
)

type DBCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
	Username string `json:"username"`
	Password string `json:"password"`
	Engine   string `json:"engine"`
}

func (c *DBCredentials) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=required",
		c.Host, c.Port, c.DBName, c.Username, c.Password,
	)
}

type cachedSecret struct {
	value     string
	fetchedAt time.Time
}

type Manager struct {
	client           *secretsmanager.Client
	rotationInterval time.Duration
	log              *logger.Logger

	mu    sync.RWMutex
	cache map[string]cachedSecret
}

func New(awsCfg aws.Config, rotationInterval time.Duration, log *logger.Logger) *Manager {
	client := secretsmanager.NewFromConfig(awsCfg)
	return &Manager{
		client:           client,
		rotationInterval: rotationInterval,
		log:              log,
		cache:            make(map[string]cachedSecret),
	}
}

func (m *Manager) GetString(ctx context.Context, secretName string) (string, error) {
	if cached, ok := m.fromCache(secretName); ok {
		return cached, nil
	}
	return m.fetchAndCache(ctx, secretName)
}

func (m *Manager) GetDBCredentials(ctx context.Context, secretName string) (*DBCredentials, error) {
	raw, err := m.GetString(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("secrets: get db credentials %q: %w", secretName, err)
	}

	var creds DBCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("secrets: unmarshal db credentials %q: %w", secretName, err)
	}

	if creds.Host == "" || creds.Username == "" {
		return nil, fmt.Errorf("secrets: db credentials %q are incomplete", secretName)
	}

	return &creds, nil
}

func (m *Manager) GetAPIKeys(ctx context.Context, secretName string) (map[string]string, error) {
	raw, err := m.GetString(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("secrets: get api keys %q: %w", secretName, err)
	}

	var keys map[string]string
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil, fmt.Errorf("secrets: unmarshal api keys %q: %w", secretName, err)
	}
	return keys, nil
}

func (m *Manager) Invalidate(secretName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, secretName)
	m.log.Info().Str("secret_name", secretName).Msg("secrets: cache invalidated")
}

func (m *Manager) StartRotationWatcher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.rotationInterval / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				m.log.Info().Msg("secrets: rotation watcher stopped")
				return
			case <-ticker.C:
				m.refreshStale(ctx)
			}
		}
	}()

	m.log.Info().
		Dur("interval", m.rotationInterval).
		Msg("secrets: rotation watcher started")
}

func (m *Manager) fromCache(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.cache[name]
	if !ok {
		return "", false
	}
	if time.Since(entry.fetchedAt) > m.rotationInterval {
		return "", false // stale data
	}
	return entry.value, true
}

func (m *Manager) fetchAndCache(ctx context.Context, name string) (string, error) {
	m.log.Debug().Str("secret_name", name).Msg("secrets: fetching from AWS")

	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)}
	out, err := m.client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("secrets: GetSecretValue %q: %w", name, err)
	}

	var value string
	switch {
	case out.SecretString != nil:
		value = *out.SecretString
	case out.SecretBinary != nil:
		value = string(out.SecretBinary)
	default:
		return "", fmt.Errorf("secrets: %q returned empty value", name)
	}

	m.mu.Lock()
	m.cache[name] = cachedSecret{value: value, fetchedAt: time.Now()}
	m.mu.Unlock()

	m.log.Debug().Str("secret_name", name).Msg("secrets: cached successfully")
	return value, nil
}

func (m *Manager) refreshStale(ctx context.Context) {
	m.mu.RLock()
	names := make([]string, 0, len(m.cache))
	for name, entry := range m.cache {
		if time.Since(entry.fetchedAt) > m.rotationInterval/2 {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()

	for _, name := range names {
		if _, err := m.fetchAndCache(ctx, name); err != nil {
			m.log.Error().Err(err).Str("secret_name", name).
				Msg("secrets: background refresh failed")
		}
	}
}
