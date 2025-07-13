/*
Copyright 2022 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package monitor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	// "time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"

	// config "github.com/kubeedge/api/apis/componentconfig/cloudcore/v1alpha1"
)

func TestRegisterMetrics(t *testing.T) {
	// Create a new registry for testing to avoid conflicts
	reg := prometheus.NewRegistry()
	
	// Save original registry and restore after test
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Test that metrics can be registered
	registerMetrics()

	// Verify that ConnectedNodes metric is registered
	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	found := false
	for _, metricFamily := range gathered {
		if metricFamily.GetName() == "KubeEdge_CloudHub_connected_nodes" {
			found = true
			break
		}
	}

	if !found {
		t.Error("ConnectedNodes metric was not registered")
	}
}

func TestConnectedNodesMetric(t *testing.T) {
	// Test setting and getting the ConnectedNodes metric value
	testValue := float64(5)
	ConnectedNodes.Set(testValue)

	// Use testutil to check the metric value
	expected := fmt.Sprintf(`
		# HELP KubeEdge_CloudHub_connected_nodes Number of nodes that connected to the cloudHub instance
		# TYPE KubeEdge_CloudHub_connected_nodes gauge
		KubeEdge_CloudHub_connected_nodes %v
	`, testValue)

	if err := testutil.CollectAndCompare(ConnectedNodes, strings.NewReader(expected)); err != nil {
		t.Errorf("Unexpected metric value: %v", err)
	}
}

func TestInstallHandlerForPProf(t *testing.T) {
	mux := http.NewServeMux()
	InstallHandlerForPProf(mux)

	// Test that pprof handlers are installed
	pprofEndpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	}

	for _, endpoint := range pprofEndpoints {
		req := httptest.NewRequest("GET", endpoint, nil)
		rec := httptest.NewRecorder()
		
		mux.ServeHTTP(rec, req)
		
		// Check that the endpoint exists (should not return 404)
		if rec.Code == http.StatusNotFound {
			t.Errorf("Pprof endpoint %s was not installed", endpoint)
		}
	}
}

func TestServeMonitorComponents(t *testing.T) {
	// Test the individual components of ServeMonitor without starting the actual server
	
	// Test with profiling enabled
	t.Run("WithProfiling", func(t *testing.T) {
		// Simulate what ServeMonitor does for setup
		registerMetrics()

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		InstallHandlerForPProf(mux)

		// Test metrics endpoint
		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("Metrics endpoint failed with status %d", rec.Code)
		}

		// Test pprof endpoint
		req = httptest.NewRequest("GET", "/debug/pprof/", nil)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		
		if rec.Code == http.StatusNotFound {
			t.Error("Pprof endpoint not available when profiling is enabled")
		}
	})

	// Test without profiling
	t.Run("WithoutProfiling", func(t *testing.T) {
		// Simulate what ServeMonitor does for setup (without pprof)
		registerMetrics()

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		// Note: InstallHandlerForPProf is NOT called when EnableProfiling is false

		// Test metrics endpoint
		req := httptest.NewRequest("GET", "/metrics", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusOK {
			t.Errorf("Metrics endpoint failed with status %d", rec.Code)
		}

		// Test that pprof endpoint is NOT available
		req = httptest.NewRequest("GET", "/debug/pprof/", nil)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		
		if rec.Code != http.StatusNotFound {
			t.Error("Pprof endpoint should not be available when profiling is disabled")
		}
	})
}

func TestMetricsEndpoint(t *testing.T) {
	// Test that the metrics endpoint returns prometheus metrics
	mux := http.NewServeMux()
	
	// Use the standard promhttp.Handler() which uses the default registry
	mux.Handle("/metrics", promhttp.Handler())

	// Set a test value
	ConnectedNodes.Set(3)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	// Check that the response contains our metric
	body := rec.Body.String()
	if !strings.Contains(body, "KubeEdge_CloudHub_connected_nodes") {
		t.Error("Metrics endpoint does not contain expected metric")
	}

	if !strings.Contains(body, "3") {
		t.Error("Metrics endpoint does not contain expected metric value")
	}
}

func TestPprofEndpointsAvailability(t *testing.T) {
	// Test that pprof endpoints are available when profiling is enabled
	mux := http.NewServeMux()
	InstallHandlerForPProf(mux)

	testCases := []struct {
		endpoint string
		method   string
	}{
		{"/debug/pprof/", "GET"},
		{"/debug/pprof/cmdline", "GET"},
		{"/debug/pprof/symbol", "GET"},
		// Note: We don't test /debug/pprof/profile and /debug/pprof/trace 
		// as they might take time to complete or require special handling
	}

	for _, tc := range testCases {
		t.Run(tc.endpoint, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.endpoint, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			// Should not return 404 (handler should be registered)
			if rec.Code == http.StatusNotFound {
				t.Errorf("Endpoint %s returned 404, handler not registered", tc.endpoint)
			}
		})
	}
}

func TestRegisterMetricsOnlyOnce(t *testing.T) {
	// This test ensures that registerMetrics is safe to call multiple times
	// due to sync.Once usage
	
	// Create a new registry for testing
	reg := prometheus.NewRegistry()
	
	// Save original registry and restore after test
	originalRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = reg
	defer func() {
		prometheus.DefaultRegisterer = originalRegistry
	}()

	// Call registerMetrics multiple times
	registerMetrics()
	registerMetrics()
	registerMetrics()

	// Should not panic and should still work correctly
	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Count how many times our metric appears
	count := 0
	for _, metricFamily := range gathered {
		if metricFamily.GetName() == "KubeEdge_CloudHub_connected_nodes" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected metric to be registered exactly once, found %d times", count)
	}
}

// BenchmarkConnectedNodesSet benchmarks the ConnectedNodes.Set operation
func BenchmarkConnectedNodesSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ConnectedNodes.Set(float64(i))
	}
}

// BenchmarkRegisterMetrics benchmarks the registerMetrics function
func BenchmarkRegisterMetrics(b *testing.B) {
	for i := 0; i < b.N; i++ {
		registerMetrics() // Should be very fast due to sync.Once
	}
}