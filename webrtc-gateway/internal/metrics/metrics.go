package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Gauges
var (
	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "whats_gateway_active_sessions",
		Help: "Number of active WebRTC sessions",
	})
	ActiveActions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "whats_gateway_active_actions",
		Help: "Number of in-flight enunciate actions",
	})
	InferenceSemUsed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "whats_gateway_inference_sem_used",
		Help: "Number of inference semaphore slots currently in use",
	})
)

// Counters
var (
	SessionsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_sessions_created_total",
		Help: "Total sessions created",
	})
	SessionsRejectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_sessions_rejected_total",
		Help: "Sessions rejected due to capacity limit",
	})
	ActionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "whats_gateway_actions_total",
		Help: "Total enunciate actions by outcome",
	}, []string{"outcome"})
	DecodeErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_opus_decode_errors_total",
		Help: "Total Opus decode failures",
	})
	EncodeErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_opus_encode_errors_total",
		Help: "Total Opus encode failures",
	})
	InferenceTimeoutsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_inference_timeouts_total",
		Help: "Total inference calls that timed out",
	})
	RTPPacketsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_rtp_packets_total",
		Help: "Total RTP packets received across all sessions",
	})
	RTPGapsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "whats_gateway_rtp_gaps_total",
		Help: "Total RTP sequence number gaps detected",
	})
)

// Histograms
var (
	ActionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "whats_gateway_action_duration_ms",
		Help:    "Enunciate action duration in milliseconds by stage",
		Buckets: []float64{100, 250, 500, 1000, 2000, 5000, 10000, 30000},
	}, []string{"stage"})
)
