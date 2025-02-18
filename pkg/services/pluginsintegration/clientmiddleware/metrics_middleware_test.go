package clientmiddleware

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/plugins/manager/client/clienttest"
	"github.com/grafana/grafana/pkg/plugins/manager/fakes"
	"github.com/grafana/grafana/pkg/plugins/pluginrequestmeta"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
)

const (
	pluginID = "plugin-id"

	metricRequestTotal      = "grafana_plugin_request_total"
	metricRequestDurationMs = "grafana_plugin_request_duration_milliseconds"
	metricRequestDurationS  = "grafana_plugin_request_duration_seconds"
	metricRequestSize       = "grafana_plugin_request_size_bytes"
)

func TestInstrumentationMiddleware(t *testing.T) {
	pCtx := backend.PluginContext{PluginID: pluginID}
	t.Run("should instrument requests", func(t *testing.T) {
		for _, tc := range []struct {
			expEndpoint                 string
			fn                          func(cdt *clienttest.ClientDecoratorTest) error
			shouldInstrumentRequestSize bool
		}{
			{
				expEndpoint: endpointCheckHealth,
				fn: func(cdt *clienttest.ClientDecoratorTest) error {
					_, err := cdt.Decorator.CheckHealth(context.Background(), &backend.CheckHealthRequest{PluginContext: pCtx})
					return err
				},
				shouldInstrumentRequestSize: false,
			},
			{
				expEndpoint: endpointCallResource,
				fn: func(cdt *clienttest.ClientDecoratorTest) error {
					return cdt.Decorator.CallResource(context.Background(), &backend.CallResourceRequest{PluginContext: pCtx}, nopCallResourceSender)
				},
				shouldInstrumentRequestSize: true,
			},
			{
				expEndpoint: endpointQueryData,
				fn: func(cdt *clienttest.ClientDecoratorTest) error {
					_, err := cdt.Decorator.QueryData(context.Background(), &backend.QueryDataRequest{PluginContext: pCtx})
					return err
				},
				shouldInstrumentRequestSize: true,
			},
			{
				expEndpoint: endpointCollectMetrics,
				fn: func(cdt *clienttest.ClientDecoratorTest) error {
					_, err := cdt.Decorator.CollectMetrics(context.Background(), &backend.CollectMetricsRequest{PluginContext: pCtx})
					return err
				},
				shouldInstrumentRequestSize: false,
			},
		} {
			t.Run(tc.expEndpoint, func(t *testing.T) {
				promRegistry := prometheus.NewRegistry()
				pluginsRegistry := fakes.NewFakePluginRegistry()
				require.NoError(t, pluginsRegistry.Add(context.Background(), &plugins.Plugin{
					JSONData: plugins.JSONData{ID: pluginID, Backend: true},
				}))

				mw := newMetricsMiddleware(promRegistry, pluginsRegistry, featuremgmt.WithFeatures())
				cdt := clienttest.NewClientDecoratorTest(t, clienttest.WithMiddlewares(
					plugins.ClientMiddlewareFunc(func(next plugins.Client) plugins.Client {
						mw.next = next
						return mw
					}),
				))
				require.NoError(t, tc.fn(cdt))

				// Ensure the correct metrics have been incremented/observed
				require.Equal(t, 1, testutil.CollectAndCount(promRegistry, metricRequestTotal))
				require.Equal(t, 1, testutil.CollectAndCount(promRegistry, metricRequestDurationMs))
				require.Equal(t, 1, testutil.CollectAndCount(promRegistry, metricRequestDurationS))

				counter := mw.pluginMetrics.pluginRequestCounter.WithLabelValues(pluginID, tc.expEndpoint, statusOK, string(backendplugin.TargetUnknown))
				require.Equal(t, 1.0, testutil.ToFloat64(counter))
				for _, m := range []string{metricRequestDurationMs, metricRequestDurationS} {
					require.NoError(t, checkHistogram(promRegistry, m, map[string]string{
						"plugin_id": pluginID,
						"endpoint":  tc.expEndpoint,
						"target":    string(backendplugin.TargetUnknown),
					}))
				}
				if tc.shouldInstrumentRequestSize {
					require.Equal(t, 1, testutil.CollectAndCount(promRegistry, metricRequestSize), "request size should have been instrumented")
					require.NoError(t, checkHistogram(promRegistry, metricRequestSize, map[string]string{
						"plugin_id": pluginID,
						"endpoint":  tc.expEndpoint,
						"target":    string(backendplugin.TargetUnknown),
						"source":    "grafana-backend",
					}), "request size should have been instrumented")
				}
			})
		}
	})
}

func TestInstrumentationMiddlewareStatusSource(t *testing.T) {
	const labelStatusSource = "status_source"
	queryDataCounterLabels := prometheus.Labels{
		"plugin_id": pluginID,
		"endpoint":  endpointQueryData,
		"status":    statusOK,
		"target":    string(backendplugin.TargetUnknown),
	}
	downstreamErrorResponse := backend.DataResponse{
		Frames:      nil,
		Error:       errors.New("bad gateway"),
		Status:      502,
		ErrorSource: backend.ErrorSourceDownstream,
	}
	pluginErrorResponse := backend.DataResponse{
		Frames:      nil,
		Error:       errors.New("internal error"),
		Status:      500,
		ErrorSource: backend.ErrorSourcePlugin,
	}
	legacyErrorResponse := backend.DataResponse{
		Frames:      nil,
		Error:       errors.New("internal error"),
		Status:      500,
		ErrorSource: "",
	}
	okResponse := backend.DataResponse{
		Frames:      nil,
		Error:       nil,
		Status:      200,
		ErrorSource: "",
	}

	pCtx := backend.PluginContext{PluginID: pluginID}

	promRegistry := prometheus.NewRegistry()
	pluginsRegistry := fakes.NewFakePluginRegistry()
	require.NoError(t, pluginsRegistry.Add(context.Background(), &plugins.Plugin{
		JSONData: plugins.JSONData{ID: pluginID, Backend: true},
	}))
	features := featuremgmt.WithFeatures(featuremgmt.FlagPluginsInstrumentationStatusSource)
	metricsMw := newMetricsMiddleware(promRegistry, pluginsRegistry, features)
	cdt := clienttest.NewClientDecoratorTest(t, clienttest.WithMiddlewares(
		NewPluginRequestMetaMiddleware(),
		plugins.ClientMiddlewareFunc(func(next plugins.Client) plugins.Client {
			metricsMw.next = next
			return metricsMw
		}),
		NewStatusSourceMiddleware(),
	))

	t.Run("Metrics", func(t *testing.T) {
		t.Run("Should ignore ErrorSource if feature flag is disabled", func(t *testing.T) {
			// Use different middleware without feature flag
			metricsMw := newMetricsMiddleware(prometheus.NewRegistry(), pluginsRegistry, featuremgmt.WithFeatures())
			cdt := clienttest.NewClientDecoratorTest(t, clienttest.WithMiddlewares(
				plugins.ClientMiddlewareFunc(func(next plugins.Client) plugins.Client {
					metricsMw.next = next
					return metricsMw
				}),
			))

			cdt.TestClient.QueryDataFunc = func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
				return &backend.QueryDataResponse{Responses: map[string]backend.DataResponse{"A": downstreamErrorResponse}}, nil
			}
			_, err := cdt.Decorator.QueryData(context.Background(), &backend.QueryDataRequest{PluginContext: pCtx})
			require.NoError(t, err)
			counter, err := metricsMw.pluginMetrics.pluginRequestCounter.GetMetricWith(newLabels(queryDataCounterLabels, nil))
			require.NoError(t, err)
			require.Equal(t, 1.0, testutil.ToFloat64(counter))

			// error_source should not be defined at all
			_, err = metricsMw.pluginMetrics.pluginRequestCounter.GetMetricWith(newLabels(
				queryDataCounterLabels,
				prometheus.Labels{
					labelStatusSource: string(backend.ErrorSourceDownstream),
				}),
			)
			require.Error(t, err)
			require.ErrorContains(t, err, "inconsistent label cardinality")
		})

		t.Run("Should add error_source label if feature flag is enabled", func(t *testing.T) {
			metricsMw.pluginMetrics.pluginRequestCounter.Reset()

			cdt.TestClient.QueryDataFunc = func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
				return &backend.QueryDataResponse{Responses: map[string]backend.DataResponse{"A": downstreamErrorResponse}}, nil
			}
			_, err := cdt.Decorator.QueryData(context.Background(), &backend.QueryDataRequest{PluginContext: pCtx})
			require.NoError(t, err)
			counter, err := metricsMw.pluginMetrics.pluginRequestCounter.GetMetricWith(newLabels(
				queryDataCounterLabels,
				prometheus.Labels{
					labelStatusSource: string(backend.ErrorSourceDownstream),
				}),
			)
			require.NoError(t, err)
			require.Equal(t, 1.0, testutil.ToFloat64(counter))
		})
	})

	t.Run("Priority", func(t *testing.T) {
		for _, tc := range []struct {
			name            string
			responses       map[string]backend.DataResponse
			expStatusSource pluginrequestmeta.StatusSource
		}{
			{
				"Default status source for ok responses should be plugin",
				map[string]backend.DataResponse{"A": okResponse},
				pluginrequestmeta.StatusSourcePlugin,
			},
			{
				"Plugin errors should have higher priority than downstream errors",
				map[string]backend.DataResponse{
					"A": pluginErrorResponse,
					"B": downstreamErrorResponse,
				},
				pluginrequestmeta.StatusSourcePlugin,
			},
			{
				"Errors without ErrorSource should be reported as plugin status source",
				map[string]backend.DataResponse{"A": legacyErrorResponse},
				pluginrequestmeta.StatusSourcePlugin,
			},
			{
				"Downstream errors should have higher priority than ok responses",
				map[string]backend.DataResponse{
					"A": okResponse,
					"B": downstreamErrorResponse,
				},
				pluginrequestmeta.StatusSourceDownstream,
			},
			{
				"Plugin errors should have higher priority than ok responses",
				map[string]backend.DataResponse{
					"A": okResponse,
					"B": pluginErrorResponse,
				},
				pluginrequestmeta.StatusSourcePlugin,
			},
			{
				"Legacy errors should have higher priority than ok responses",
				map[string]backend.DataResponse{
					"A": okResponse,
					"B": legacyErrorResponse,
				},
				pluginrequestmeta.StatusSourcePlugin,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Cleanup(func() {
					cdt.QueryDataCtx = nil
					cdt.QueryDataReq = nil
				})
				cdt.TestClient.QueryDataFunc = func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
					cdt.QueryDataCtx = ctx
					cdt.QueryDataReq = req
					return &backend.QueryDataResponse{Responses: tc.responses}, nil
				}
				_, err := cdt.Decorator.QueryData(context.Background(), &backend.QueryDataRequest{PluginContext: pCtx})
				require.NoError(t, err)
				ctxStatusSource := pluginrequestmeta.StatusSourceFromContext(cdt.QueryDataCtx)
				require.Equal(t, tc.expStatusSource, ctxStatusSource)
			})
		}
	})
}

// checkHistogram is a utility function that checks if a histogram with the given name and label values exists
// and has been observed at least once.
func checkHistogram(promRegistry *prometheus.Registry, expMetricName string, expLabels map[string]string) error {
	metrics, err := promRegistry.Gather()
	if err != nil {
		return fmt.Errorf("gather: %w", err)
	}
	var metricFamily *dto.MetricFamily
	for _, mf := range metrics {
		if *mf.Name == expMetricName {
			metricFamily = mf
			break
		}
	}
	if metricFamily == nil {
		return fmt.Errorf("metric %q not found", expMetricName)
	}
	var foundLabels int
	var metric *dto.Metric
	for _, m := range metricFamily.Metric {
		for _, l := range m.GetLabel() {
			v, ok := expLabels[*l.Name]
			if !ok {
				continue
			}
			if v != *l.Value {
				return fmt.Errorf("expected label %q to have value %q, got %q", *l.Name, v, *l.Value)
			}
			foundLabels++
		}
		if foundLabels == 0 {
			continue
		}
		if foundLabels != len(expLabels) {
			return fmt.Errorf("expected %d labels, got %d", len(expLabels), foundLabels)
		}
		metric = m
		break
	}
	if metric == nil {
		return fmt.Errorf("could not find metric with labels %v", expLabels)
	}
	if metric.Histogram == nil {
		return fmt.Errorf("metric %q is not a histogram", expMetricName)
	}
	if metric.Histogram.SampleCount == nil || *metric.Histogram.SampleCount == 0 {
		return errors.New("found metric but no samples have been collected")
	}
	return nil
}

// newLabels creates a new prometheus.Labels from the given initial labels and additional labels.
// The additionalLabels are merged into the initial ones, and will overwrite a value if already set in initialLabels.
func newLabels(initialLabels prometheus.Labels, additional ...prometheus.Labels) prometheus.Labels {
	r := make(prometheus.Labels, len(initialLabels)+len(additional)/2)
	for k, v := range initialLabels {
		r[k] = v
	}
	for _, l := range additional {
		for k, v := range l {
			r[k] = v
		}
	}
	return r
}
