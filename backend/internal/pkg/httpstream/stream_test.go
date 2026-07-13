package httpstream

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/selfevo-AI/meta-org-saas/backend/internal/pkg/middleware"
)

func TestPrepareAllowsStreamingBeyondServerWriteTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{
		WriteTimeout: 40 * time.Millisecond,
		Handler: middleware.APIErrorContract(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if err := Prepare(w); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			_, _ = fmt.Fprint(w, "data: first\n\n")
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
			_, _ = fmt.Fprint(w, "data: second\n\n")
			flusher.Flush()
		})),
	}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	response, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	var events []string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data:") {
			events = append(events, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(events) != 2 || events[0] != "data: first" || events[1] != "data: second" {
		t.Fatalf("events = %#v", events)
	}
}
