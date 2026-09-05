package main

import (
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const healthTimeout = 10 * time.Second

// defaultFallbackURL is where visitors are sent during a total outage when the
// operator has not configured FALLBACK_URL. The project's own page is the least
// confusing landing spot: it explains what detour is, which is about all that
// can be said when every backend is down.
const defaultFallbackURL = "https://github.com/skorokithakis/detour"

// redirectDelaySeconds is how long the no-backend page waits before sending the
// visitor to the fallback URL. It is deliberately not configurable: five
// seconds is enough to read the page and short enough not to strand anyone.
const redirectDelaySeconds = 5

var healthClient = &http.Client{
	Timeout: healthTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type backend struct {
	base      string
	healthURL string
}

type config struct {
	backends         []*backend
	fallbackURL      string
	healthyThreshold int
	interval         time.Duration
	port             string
}

type healthChecker struct {
	backends     []*backend
	successCount []int
	threshold    int
	// firstCheck is true until the first check has run. The streak counters
	// alone cannot distinguish "never checked" from "failed the last check",
	// so the first check needs its own flag to report the state of every
	// backend rather than just transitions.
	firstCheck bool
	active     *atomic.Pointer[backend]
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	active := &atomic.Pointer[backend]{}
	checker := healthChecker{
		backends:     config.backends,
		successCount: make([]int, len(config.backends)),
		threshold:    config.healthyThreshold,
		firstCheck:   true,
		active:       active,
	}

	log.Printf("listening on port %s with %d backends; check interval %s, healthy threshold %d", config.port, len(config.backends), config.interval, config.healthyThreshold)
	go checker.run(config.interval)

	unavailablePage := buildUnavailablePage(config.fallbackURL)

	server := &http.Server{
		Addr:              net.JoinHostPort("", config.port),
		Handler:           redirectHandler(active, unavailablePage),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func loadConfig() (config, error) {
	rawBackends := os.Getenv("BACKENDS")
	if strings.TrimSpace(rawBackends) == "" {
		return config{}, fmt.Errorf("BACKENDS must not be empty")
	}

	backends := make([]*backend, 0, len(strings.Split(rawBackends, ",")))
	for _, rawBackend := range strings.Split(rawBackends, ",") {
		backend, err := parseBackend(strings.TrimSpace(rawBackend))
		if err != nil {
			return config{}, fmt.Errorf("invalid backend %q: %w", rawBackend, err)
		}
		backends = append(backends, backend)
	}

	intervalValue := os.Getenv("CHECK_INTERVAL")
	if intervalValue == "" {
		intervalValue = "60s"
	}
	interval, err := time.ParseDuration(intervalValue)
	if err != nil || interval <= 0 {
		return config{}, fmt.Errorf("invalid CHECK_INTERVAL %q", intervalValue)
	}

	thresholdValue := os.Getenv("HEALTHY_THRESHOLD")
	if thresholdValue == "" {
		thresholdValue = "3"
	}
	healthyThreshold, err := strconv.Atoi(thresholdValue)
	if err != nil || healthyThreshold < 1 {
		return config{}, fmt.Errorf("invalid HEALTHY_THRESHOLD %q", thresholdValue)
	}

	fallbackValue := os.Getenv("FALLBACK_URL")
	if fallbackValue == "" {
		fallbackValue = defaultFallbackURL
	}
	// url.Parse, not parseBackend: the fallback URL may carry a query string
	// or fragment, and ParseRequestURI leaves fragments glued to the query.
	parsedFallback, err := url.Parse(fallbackValue)
	if err != nil || (parsedFallback.Scheme != "http" && parsedFallback.Scheme != "https") || parsedFallback.Hostname() == "" {
		return config{}, fmt.Errorf("invalid FALLBACK_URL %q", fallbackValue)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return config{backends: backends, fallbackURL: fallbackValue, healthyThreshold: healthyThreshold, interval: interval, port: port}, nil
}

func parseBackend(raw string) (*backend, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("must be an absolute HTTP(S) URL")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return nil, fmt.Errorf("must not include a query string or fragment")
	}

	base := parsed.String()
	healthURL := base
	if !strings.HasSuffix(healthURL, "/") {
		healthURL += "/"
	}
	return &backend{base: base, healthURL: healthURL}, nil
}

func (checker *healthChecker) run(interval time.Duration) {
	checker.check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		checker.check()
	}
}

func (checker *healthChecker) check() {
	results := make([]bool, len(checker.backends))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(checker.backends))
	for index, checkedBackend := range checker.backends {
		go func(index int, checkedBackend *backend) {
			defer waitGroup.Done()
			results[index] = isLive(checkedBackend.healthURL)
		}(index, checkedBackend)
	}
	waitGroup.Wait()

	for index, backend := range checker.backends {
		// The up/down log follows the raw probe result rather than the damped
		// streak, so a flapping backend stays visible to operators. The first
		// check logs every backend, up or down, so the startup state of the
		// fleet is visible; afterwards only transitions are logged.
		if results[index] && (checker.firstCheck || checker.successCount[index] == 0) {
			log.Printf("backend %s is up", backend.base)
		}
		if !results[index] && (checker.firstCheck || checker.successCount[index] > 0) {
			log.Printf("backend %s is down", backend.base)
		}
		if results[index] {
			checker.successCount[index]++
		} else {
			checker.successCount[index] = 0
		}
	}

	var next *backend
	if chosen := selectBackendIndex(checker.successCount, checker.threshold); chosen != -1 {
		next = checker.backends[chosen]
	}

	if previous := checker.active.Load(); previous != next {
		checker.active.Store(next)
		if next == nil {
			log.Printf("active backend changed to none")
		} else {
			log.Printf("active backend changed to %s", next.base)
		}
	}

	checker.firstCheck = false
}

// selectBackendIndex returns the index of the backend that should hold the
// active slot, in priority order, or -1 when no backend has succeeded even once.
//
// The second pass is deliberate. Hysteresis exists to avoid unnecessary
// switching, but with no backend at the threshold a marginal backend still beats
// a 503. Without the second pass, cold start and recovery from a total outage
// would each cost threshold x CHECK_INTERVAL of downtime.
func selectBackendIndex(successCounts []int, threshold int) int {
	for index, count := range successCounts {
		if count >= threshold {
			return index
		}
	}
	for index, count := range successCounts {
		if count >= 1 {
			return index
		}
	}
	return -1
}

func isLive(healthURL string) bool {
	response, err := healthClient.Get(healthURL)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// buildUnavailablePage renders the page served when every backend is down. The
// page depends only on configuration, so it is rendered once at startup instead
// of being rebuilt on every request of an outage. The redirect is a meta
// refresh rather than a script or a Refresh header so the page still works with
// scripting disabled and only uses standard behaviour.
func buildUnavailablePage(fallbackURL string) string {
	// The URL is operator configuration rather than user input, but a quote in
	// it would still break out of the attribute it lands in, so every
	// interpolation goes through EscapeString.
	escapedFallbackURL := html.EscapeString(fallbackURL)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="%d;url=%s">
<title>No healthy backends</title>
<style>
body { margin: 0; min-height: 100vh; display: grid; place-items: center; font-family: sans-serif; }
main { text-align: center; padding: 0 1rem; }
</style>
</head>
<body>
<main>
<h1>No healthy backends</h1>
<p>This service is unavailable. You will be redirected in %d seconds.</p>
<p>If you are not redirected automatically, go to <a href="%s">%s</a>.</p>
</main>
</body>
</html>
`, redirectDelaySeconds, escapedFallbackURL, redirectDelaySeconds, escapedFallbackURL, escapedFallbackURL)
}

func redirectHandler(active *atomic.Pointer[backend], unavailablePage string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		backend := active.Load()
		if backend == nil {
			// The status must stay 503 even though the page is human-readable:
			// the service genuinely is unavailable and a 200 here would hide
			// the outage from uptime monitoring. A meta refresh works inside a
			// 503 body, so visitors are still sent onward.
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(writer, unavailablePage)
			return
		}

		writer.Header().Set("Location", joinURL(backend.base, request.URL.RequestURI()))
		writer.WriteHeader(http.StatusFound)
	})
}

func joinURL(base, requestURI string) string {
	if strings.HasSuffix(base, "/") && strings.HasPrefix(requestURI, "/") {
		return base + requestURI[1:]
	}
	if !strings.HasSuffix(base, "/") && !strings.HasPrefix(requestURI, "/") {
		return base + "/" + requestURI
	}
	return base + requestURI
}
