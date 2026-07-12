package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testLifecyclePolicy(t *testing.T) LifecyclePolicy {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "workspace")
	return LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
}

func initializedLifecycle(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy) {
	t.Helper()
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	return s, child, bufio.NewScanner(child.stdinR), testLifecyclePolicy(t)
}

func lifecycleRequest(t *testing.T, scanner *bufio.Scanner, wantMethod string, wantID uint64) map[string]any {
	t.Helper()
	if !scanner.Scan() {
		t.Fatalf("expected %s request", wantMethod)
	}
	var request map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	if request["method"] != wantMethod || request["id"] != float64(wantID) {
		t.Fatalf("request = %#v, want %s/%d", request, wantMethod, wantID)
	}
	return request
}

func requestParams(t *testing.T, request map[string]any) map[string]any {
	t.Helper()
	params, ok := request["params"].(map[string]any)
	if !ok {
		t.Fatalf("request has no parameters: %#v", request)
	}
	return params
}

func assertPinnedPolicy(t *testing.T, request map[string]any, policy LifecyclePolicy) {
	t.Helper()
	params := requestParams(t, request)
	if params["cwd"] != policy.Workspace || params["sandbox"] != "workspace-write" || params["approvalPolicy"] != "untrusted" || params["approvalsReviewer"] != "user" || params["model"] != PinnedModel || params["effort"] != PinnedReasoningEffort || params["modelProvider"] != "openai" {
		t.Fatalf("request does not retain pinned policy: %#v", request)
	}
	sandbox, ok := params["sandboxPolicy"].(map[string]any)
	if !ok || sandbox["type"] != "workspaceWrite" || sandbox["networkAccess"] != false {
		t.Fatalf("sandbox policy = %#v", request["sandboxPolicy"])
	}
	roots, ok := sandbox["writableRoots"].([]any)
	if !ok || len(roots) != 1 || roots[0] != policy.Workspace {
		t.Fatalf("writable roots = %#v", sandbox["writableRoots"])
	}
}

func startThreadForTest(t *testing.T, s *Supervisor, child *fakeChild, scanner *bufio.Scanner, policy LifecyclePolicy) LifecycleReference {
	t.Helper()
	done := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartThread(context.Background(), policy)
		done <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	request := lifecycleRequest(t, scanner, "thread/start", 2)
	assertPinnedPolicy(t, request, policy)
	_, _ = child.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-1"}}`)))
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	return result.ref
}

func TestLifecycleInitializedThreadReadResumeTurnAndSteer(t *testing.T) {
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	for _, operation := range []struct {
		method string
		call   func() (LifecycleReference, error)
	}{
		{"thread/read", func() (LifecycleReference, error) { return s.ReadThread(context.Background(), thread, policy) }},
		{"thread/resume", func() (LifecycleReference, error) { return s.ResumeThread(context.Background(), thread, policy) }},
	} {
		done := make(chan error, 1)
		go func() { _, err := operation.call(); done <- err }()
		request := lifecycleRequest(t, scanner, operation.method, map[string]uint64{"thread/read": 3, "thread/resume": 4}[operation.method])
		assertPinnedPolicy(t, request, policy)
		params := requestParams(t, request)
		if params["threadId"] != "thread-1" || params["includeTurns"] != false {
			t.Fatalf("thread lifecycle params = %#v", request)
		}
		_, _ = child.stdoutW.Write([]byte(response(map[string]uint64{"thread/read": 3, "thread/resume": 4}[operation.method], `{"thread":{"id":"thread-1"}}`)))
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	turnDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		turnDone <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	turnRequest := lifecycleRequest(t, scanner, "turn/start", 5)
	assertPinnedPolicy(t, turnRequest, policy)
	if requestParams(t, turnRequest)["threadId"] != "thread-1" {
		t.Fatalf("turn request = %#v", turnRequest)
	}
	_, _ = child.stdoutW.Write([]byte(response(5, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-turnDone
	if turn.err != nil || turn.ref.Session.SessionID != "thread-1" || turn.ref.Correlation.InteractionID != "turn-1" {
		t.Fatalf("turn projection = %+v, err=%v", turn.ref, turn.err)
	}
	steerDone := make(chan error, 1)
	go func() { steerDone <- s.SteerTurn(context.Background(), turn.ref, policy, "turn-input-2") }()
	steerRequest := lifecycleRequest(t, scanner, "turn/steer", 6)
	assertPinnedPolicy(t, steerRequest, policy)
	if requestParams(t, steerRequest)["threadId"] != "thread-1" || requestParams(t, steerRequest)["turnId"] != "turn-1" {
		t.Fatalf("steer request = %#v", steerRequest)
	}
	_, _ = child.stdoutW.Write([]byte(response(6, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	if err := <-steerDone; err != nil {
		t.Fatal(err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("initial event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "running" || event.Summary != "turn_started" {
		t.Fatalf("turn event = %+v", event)
	}
}

func TestLifecycleRejectsStaleAndDuplicateTurnReferences(t *testing.T) {
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	stale := thread
	stale.Correlation.ConnectionID = "stale"
	if _, err := s.ReadThread(context.Background(), stale, policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("stale read error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("stale event = %+v", event)
	}

	s, child, scanner, policy = initializedLifecycle(t)
	thread = startThreadForTest(t, s, child, scanner, policy)
	stale = thread
	stale.Correlation.ProcessIncarnationID = "stale"
	if _, err := s.ResumeThread(context.Background(), stale, policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("stale resume error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" || event.Summary != string(DisconnectLifecycleRejected) {
		t.Fatalf("stale resume event = %+v", event)
	}

	s, child, scanner, policy = initializedLifecycle(t)
	thread = startThreadForTest(t, s, child, scanner, policy)
	first := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1"); first <- err }()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	second := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-2"); second <- err }()
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("duplicate turn error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatalf("ready event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "running" {
		t.Fatalf("running event = %+v", event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatalf("duplicate event = %+v", event)
	}
}

func TestLifecycleSteerRejectsNonSteerableAndBadResponse(t *testing.T) {
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	done := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		done <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-done
	if turn.err != nil {
		t.Fatal(turn.err)
	}
	s.lifecycle.steerable = false
	if err := s.SteerTurn(context.Background(), turn.ref, policy, "turn-input-2"); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("non-steerable error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "running" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatal(event)
	}
}

func TestLifecycleProtocolAndDeadlineFailuresFailClosed(t *testing.T) {
	for name, fixture := range map[string]struct {
		frame  string
		reason DisconnectReason
	}{
		"malformed":      {"not-json\n", DisconnectMalformedEnvelope},
		"provider_error": {`{"jsonrpc":"2.0","id":2,"error":{"message":"private"}}` + "\n", DisconnectProviderError},
		"id_mismatch":    {response(3, `{"thread":{"id":"thread-1"}}`), DisconnectCorrelationMismatch},
		"invalid_shape":  {response(2, `{"thread":{"id":"bad id"}}`), DisconnectUnsupportedLifecycle},
		"reroute":        {response(2, `{"thread":{"id":"thread-1"},"modelRerouted":true}`), DisconnectModelRerouted},
	} {
		t.Run(name, func(t *testing.T) {
			s, child, scanner, policy := initializedLifecycle(t)
			done := make(chan error, 1)
			go func() { _, err := s.StartThread(context.Background(), policy); done <- err }()
			_ = lifecycleRequest(t, scanner, "thread/start", 2)
			_, _ = child.stdoutW.Write([]byte(fixture.frame))
			if err := <-done; err == nil {
				t.Fatal("expected lifecycle failure")
			}
			_ = nextEvent(t, s)
			event := nextEvent(t, s)
			if event.State != "disconnected" || event.Summary != string(fixture.reason) {
				t.Fatalf("event = %+v", event)
			}
		})
	}
	t.Run("deadline", func(t *testing.T) {
		s, _, scanner, policy := initializedLifecycle(t)
		s.deadlines.Request = 20 * time.Millisecond
		done := make(chan error, 1)
		go func() { _, err := s.StartThread(context.Background(), policy); done <- err }()
		_ = lifecycleRequest(t, scanner, "thread/start", 2)
		if err := <-done; err == nil {
			t.Fatal("expected deadline")
		}
		_ = nextEvent(t, s)
		event := nextEvent(t, s)
		if event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("event = %+v", event)
		}
	})
}

func TestTurnLifecycleStateIsRejected(t *testing.T) {
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	done := make(chan error, 1)
	go func() { _, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1"); done <- err }()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"completed"}}`)))
	if err := <-done; !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("turn lifecycle-state error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.Summary != string(DisconnectUnsupportedLifecycle) {
		t.Fatal(event)
	}
}

func TestLifecyclePolicyAndTransportGate(t *testing.T) {
	policy := testLifecyclePolicy(t)
	for name, mutate := range map[string]func(*LifecyclePolicy){
		"full_access": func(p *LifecyclePolicy) { p.FullAccess = true }, "shell": func(p *LifecyclePolicy) { p.AllowShell = true },
		"auto_review": func(p *LifecyclePolicy) { p.AutoReview = true }, "network": func(p *LifecyclePolicy) { p.NetworkEnabled = true },
		"no_roots": func(p *LifecyclePolicy) { p.WritableRoots = nil }, "model": func(p *LifecyclePolicy) { p.Model = "other" },
		"effort": func(p *LifecyclePolicy) { p.ReasoningEffort = "low" }, "fallback": func(p *LifecyclePolicy) { p.FallbackModel = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := policy
			mutate(&candidate)
			if candidate.validate() == nil {
				t.Fatal("unsafe policy was accepted")
			}
		})
	}
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	changed := policy
	changed.Workspace = filepath.Join(t.TempDir(), "other-workspace")
	changed.WritableRoots = []string{changed.Workspace}
	if _, err := s.ReadThread(context.Background(), thread, changed); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("changed policy error = %v", err)
	}
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.Summary != string(DisconnectPolicyMismatch) {
		t.Fatal(event)
	}

	s, child, _, policy = initializedLifecycle(t)
	child.exit(errors.New("lost"))
	if event := nextEvent(t, s); event.State != "ready" {
		t.Fatal(event)
	}
	if event := nextEvent(t, s); event.State != "disconnected" {
		t.Fatal(event)
	}
	if _, err := s.StartThread(context.Background(), policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("post-loss lifecycle error = %v", err)
	}
}
