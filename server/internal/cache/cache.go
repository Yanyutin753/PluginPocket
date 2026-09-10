// Package cache provides disposable Redis metadata and invalidation transport.
// PostgreSQL remains authoritative; callers can fall back when Redis is unavailable.
package cache

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

const operationTimeout = 250 * time.Millisecond

var ErrUnavailable = errors.New("redis cache unavailable")
var namespacePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Client struct {
	redis   *redis.Client
	prefix  string
	ctx     context.Context
	cancel  context.CancelFunc
	watcher chan struct{}
}

// Open validates configuration without requiring a live Redis server.
func Open(rawURL, namespace string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "redis" && u.Scheme != "rediss") || !namespacePattern.MatchString(namespace) {
		return nil, errors.New("invalid Redis URL or namespace")
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid Redis URL")
	}
	options.DialTimeout = operationTimeout
	options.ReadTimeout = operationTimeout
	options.WriteTimeout = operationTimeout
	options.PoolTimeout = operationTimeout
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1
	options.PoolSize = 8
	options.MaxActiveConns = 8
	options.MinIdleConns = 0
	options.MaxIdleConns = 2
	options.DisableIdentity = true
	options.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{redis: redis.NewClient(options), prefix: namespace + ":", ctx: ctx, cancel: cancel, watcher: make(chan struct{}, 1)}, nil
}

func safeError(err error) error {
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	value, err := c.redis.Get(ctx, c.prefix+"data:"+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return value, safeError(err)
}
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("cache TTL must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return safeError(c.redis.Set(ctx, c.prefix+"data:"+key, value, ttl).Err())
}
func (c *Client) PublishInvalidation(ctx context.Context, source string) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return safeError(c.redis.Publish(ctx, c.prefix+"invalidate", source).Err())
}

// Watch owns at most one subscription socket per Client. The official client
// reconnects and resubscribes; cancellation closes the subscription immediately.
func (c *Client) Watch(ctx context.Context, onMessage func(string)) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(c.ctx, cancel)
	defer stop()
	select {
	case c.watcher <- struct{}{}:
		defer func() { <-c.watcher }()
	case <-ctx.Done():
		return
	}
	subscription := c.redis.Subscribe(ctx, c.prefix+"invalidate")
	// Subscription shutdown has no result to recover; the watcher is already stopping.
	defer func() { _ = subscription.Close() }()
	messages := subscription.Channel(redis.WithChannelHealthCheckInterval(time.Second))
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok {
				return
			}
			onMessage(message.Payload)
		}
	}
}
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return safeError(c.redis.Ping(ctx).Err())
}
func (c *Client) Close() error { c.cancel(); return safeError(c.redis.Close()) }
