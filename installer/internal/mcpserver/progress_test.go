package mcpserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/review"
)

func TestProgressEmitterDebounceUndFormat(t *testing.T) {
	current := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	events := []*mcp.ProgressNotificationParams{}
	emitter := newProgressEmitter(context.Background(), "token", 2, func(_ context.Context, params *mcp.ProgressNotificationParams) error {
		events = append(events, params)
		return nil
	}, 0, time.Second, func() time.Time { return current })

	emitter.Report("gitleaks", review.JobStatus{Job: "gitleaks", State: review.StateRunning})
	emitter.Report("grype", review.JobStatus{Job: "grype", State: review.StateRunning})
	if len(events) != 1 {
		t.Fatalf("Events nach Debounce = %d, erwartet 1", len(events))
	}
	if events[0].ProgressToken != "token" || events[0].Progress != 0 || events[0].Total != 2 || events[0].Message == "" {
		t.Fatalf("Progress-Format = %+v", events[0])
	}

	current = current.Add(time.Second + time.Millisecond)
	emitter.Report("gitleaks", review.JobStatus{Job: "gitleaks", State: review.StateDone})
	if len(events) != 2 {
		t.Fatalf("Events nach Debounce-Fenster = %d, erwartet 2", len(events))
	}
	if events[1].Progress != 1 || events[1].Total != 2 {
		t.Fatalf("Fortschritt = %.0f/%.0f, erwartet 1/2", events[1].Progress, events[1].Total)
	}
}

func TestProgressEmitterHeartbeatUndStop(t *testing.T) {
	events := make(chan *mcp.ProgressNotificationParams, 8)
	emitter := newProgressEmitter(context.Background(), "token", 1, func(_ context.Context, params *mcp.ProgressNotificationParams) error {
		events <- params
		return nil
	}, 10*time.Millisecond, 0, time.Now)
	emitter.start()

	select {
	case event := <-events:
		if event.Message == "" || event.ProgressToken != "token" {
			t.Fatalf("Heartbeat-Event = %+v", event)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("kein Heartbeat-Event empfangen")
	}

	emitter.Stop("scan complete")
	for {
		select {
		case <-events:
			continue
		default:
		}
		break
	}
	emitter.Report("gitleaks", review.JobStatus{Job: "gitleaks", State: review.StateDone})

	select {
	case event := <-events:
		t.Fatalf("Event nach Stop: %+v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestProgressEmitterIstGoroutineSicher(t *testing.T) {
	var mutex sync.Mutex
	events := []*mcp.ProgressNotificationParams{}
	emitter := newProgressEmitter(context.Background(), "token", 50, func(_ context.Context, params *mcp.ProgressNotificationParams) error {
		mutex.Lock()
		defer mutex.Unlock()
		events = append(events, params)
		return nil
	}, 0, 0, time.Now)

	var group sync.WaitGroup
	for index := 0; index < 50; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			name := "entry"
			if index%2 == 0 {
				name = "entry-a"
			}
			emitter.Report(name, review.JobStatus{Job: name, State: review.StateRunning})
			emitter.Report(name, review.JobStatus{Job: name, State: review.StateDone})
		}(index)
	}
	group.Wait()
	emitter.Stop("scan complete")

	mutex.Lock()
	defer mutex.Unlock()
	if len(events) == 0 {
		t.Fatal("keine Events aus nebenläufigen Reports")
	}
	last := events[len(events)-1]
	if last.ProgressToken != "token" || last.Total != 50 {
		t.Fatalf("letztes Event = %+v", last)
	}
}

func TestNewReviewProgressEmitterOhneTokenIstNil(t *testing.T) {
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{}, Session: &mcp.ServerSession{}}
	if emitter := newReviewProgressEmitter(context.Background(), request, t.TempDir(), nil); emitter != nil {
		t.Fatalf("Emitter ohne progressToken = %#v, erwartet nil", emitter)
	}
}
