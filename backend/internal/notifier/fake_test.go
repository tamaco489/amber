package notifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// ================================================================================
// Slack の差し替え
// ================================================================================

type reaction struct {
	channel, ts, name string
}

type fakeSlack struct {
	mu        sync.Mutex
	seq       int
	posts     []slack.Message
	updates   []slack.Message
	reactions []reaction

	// メソッドの名前ごとに、次の n 回を失敗させる
	failures map[string]int
	failWith error
}

func (f *fakeSlack) fail(method string) error {
	if f.failures[method] > 0 {
		f.failures[method]--
		if f.failWith != nil {
			return f.failWith
		}
		return fmt.Errorf("fake %s failure", method)
	}
	return nil
}

func (f *fakeSlack) PostMessage(_ context.Context, msg slack.Message) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("chat.postMessage"); err != nil {
		return "", err
	}
	f.seq++
	f.posts = append(f.posts, msg)
	return fmt.Sprintf("1700000000.%06d", f.seq), nil
}

func (f *fakeSlack) UpdateMessage(_ context.Context, msg slack.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("chat.update"); err != nil {
		return err
	}
	f.updates = append(f.updates, msg)
	return nil
}

func (f *fakeSlack) AddReaction(_ context.Context, channel, ts, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("reactions.add"); err != nil {
		return err
	}
	f.reactions = append(f.reactions, reaction{channel: channel, ts: ts, name: name})
	return nil
}

// parents は、スレッドの外への投稿 (親の投稿)
func (f *fakeSlack) parents() []slack.Message {
	var out []slack.Message
	for _, m := range f.posts {
		if m.ThreadTS == "" {
			out = append(out, m)
		}
	}
	return out
}

// replies は、スレッドへの返信
func (f *fakeSlack) replies() []slack.Message {
	var out []slack.Message
	for _, m := range f.posts {
		if m.ThreadTS != "" {
			out = append(out, m)
		}
	}
	return out
}

// lastParent は、親の今の見た目 (最後の chat.update、無ければ最初の投稿)
func (f *fakeSlack) lastParent() slack.Message {
	if len(f.updates) > 0 {
		return f.updates[len(f.updates)-1]
	}
	if ps := f.parents(); len(ps) > 0 {
		return ps[0]
	}
	return slack.Message{}
}

// ================================================================================
// DynamoDB の差し替え (DynamoDBStore と同じ条件で動く)
// ================================================================================

type fakeStep struct {
	leaseUntil time.Time
	done       bool
}

type fakeStore struct {
	mu      sync.Mutex
	lease   time.Duration
	steps   map[string]*fakeStep
	parents map[string]Parent
}

func newFakeStore() *fakeStore {
	return &fakeStore{lease: defaultLeaseTime, steps: map[string]*fakeStep{}, parents: map[string]Parent{}}
}

func (s *fakeStore) Claim(_ context.Context, key string, now time.Time) (ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.steps[key]
	if !ok {
		st = &fakeStep{}
		s.steps[key] = st
	}
	if st.done {
		return ClaimDone, nil
	}
	if !st.leaseUntil.IsZero() && !st.leaseUntil.Before(now) {
		return ClaimBusy, nil
	}
	st.leaseUntil = now.Add(s.lease)
	return ClaimAcquired, nil
}

func (s *fakeStore) Done(_ context.Context, key string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps[key] = &fakeStep{done: true}
	return nil
}

func (s *fakeStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.steps[key]; ok {
		st.leaseUntil = time.Time{}
	}
	return nil
}

func (s *fakeStore) GetParent(_ context.Context, deploymentID string) (Parent, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.parents[deploymentID]
	return p, ok, nil
}

func (s *fakeStore) PutParent(_ context.Context, p Parent, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.parents[p.DeploymentID]
	if p.Rev <= 1 && ok || p.Rev > 1 && (!ok || cur.Rev != p.Rev-1) {
		return ErrConflict
	}
	s.parents[p.DeploymentID] = p
	return nil
}
