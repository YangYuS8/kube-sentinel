package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ksv1alpha1 "github.com/yangyus8/kube-sentinel/api/v1alpha1"
	"github.com/yangyus8/kube-sentinel/internal/controllers"
	"github.com/yangyus8/kube-sentinel/internal/ingestion"
	"github.com/yangyus8/kube-sentinel/internal/observability"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := run(); err != nil {
		ctrl.Log.WithName("setup").Error(err, "manager exited")
		os.Exit(1)
	}
}

func run() error {
	var metricsAddr string
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", envOrDefault("KUBE_SENTINEL_METRICS_BIND_ADDRESS", ":8080"), "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", envOrDefault("KUBE_SENTINEL_HEALTH_PROBE_BIND_ADDRESS", ":8081"), "The address the probe endpoint binds to.")
	flag.Parse()

	setupLog := ctrl.Log.WithName("setup")
	webhookAddr := envOrDefault("KUBE_SENTINEL_WEBHOOK_BIND_ADDRESS", ":8090")
	webhookPath := envOrDefault("KUBE_SENTINEL_WEBHOOK_PATH", "/alertmanager/webhook")
	observability.RegisterPrometheusMetrics()

	setupLog.Info("starting kube-sentinel manager", "metricsAddr", metricsAddr, "probeAddr", probeAddr, "webhookAddr", webhookAddr, "webhookPath", webhookPath)

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ksv1alpha1.AddToScheme(scheme))

	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("load kubernetes config: %w", err)
	}
	setupLog.Info("loaded kubernetes client configuration", "host", config.Host)

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		return fmt.Errorf("create controller manager: %w", err)
	}

	if err := (&controllers.HealingRequestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup HealingRequest controller: %w", err)
	}
	setupLog.Info("registered HealingRequest controller")

	auditSink := &observability.MemoryAuditSink{}
	receiver := &ingestion.Receiver{
		Client:    mgr.GetClient(),
		Dedupe:    ingestion.NewMemoryDedupeStore(),
		AuditSink: auditSink,
	}
	http.HandleFunc(webhookPath, receiver.HandleWebhook)
	go func() {
		setupLog.Info("starting Alertmanager webhook receiver", "addr", webhookAddr, "path", webhookPath)
		if err := http.ListenAndServe(webhookAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
			setupLog.Error(err, "webhook receiver stopped unexpectedly", "addr", webhookAddr, "path", webhookPath)
		}
	}()

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("register healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("register readyz check: %w", err)
	}
	setupLog.Info("registered health and readiness checks")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("start controller manager: %w", err)
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
