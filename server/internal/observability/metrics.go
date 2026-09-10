package observability

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func New(pool *pgxpool.Pool) *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "loadout_http_requests_total", Help: "HTTP requests by normalized method and status code."}, []string{"code", "method"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "loadout_http_request_duration_seconds", Help: "HTTP handler duration.", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30}}, []string{"method"})
	registry.MustRegister(requests, duration, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if pool != nil {
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "loadout_db_connections_in_use", Help: "Currently acquired PostgreSQL connections."}, func() float64 { return float64(pool.Stat().AcquiredConns()) }))
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "loadout_db_connections_total", Help: "Current PostgreSQL pool size."}, func() float64 { return float64(pool.Stat().TotalConns()) }))
	}
	return &Metrics{registry, requests, duration}
}
func (m *Metrics) Wrap(h http.Handler) http.Handler {
	measured := promhttp.InstrumentHandlerDuration(m.duration, promhttp.InstrumentHandlerCounter(m.requests, h))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", rand.Text())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		measured.ServeHTTP(w, r)
	})
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{MaxRequestsInFlight: 2, Timeout: 5 * time.Second})
}
