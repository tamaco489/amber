package integration_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/dynamodb"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// ================================================================================
// Slack の差し替え
// ================================================================================

type reaction struct {
	channel string // チャンネルの ID
	ts      string // スタンプを付けた投稿の ts
	name    string // スタンプの名前
}

type fakeSlack struct {
	mu        sync.Mutex
	seq       int             // 投稿の ts の連番
	posts     []slack.Message // chat.postMessage で送ったもの
	updates   []slack.Message // chat.update で送ったもの
	reactions []reaction      // reactions.add で付けたもの
	failures  map[string]int  // メソッドの名前ごとに、次の n 回を失敗させる
	failWith  error           // 失敗させるときのエラー (nil なら決まった文言)
}

func (f *fakeSlack) fail(method string) error {
	if f.failures[method] == 0 {
		return nil
	}
	f.failures[method]--
	if f.failWith != nil {
		return f.failWith
	}
	return fmt.Errorf("fake %s failure", method)
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
	ps := f.parents()
	if len(ps) == 0 {
		return slack.Message{}
	}
	return ps[0]
}

// ================================================================================
// DynamoDB の差し替え (dynamodb.Store と同じ条件で動く)
// ================================================================================

type fakeStep struct {
	leaseUntil time.Time // 処理中の印の期限
	done       bool      // 済んだ印
}

type fakeStore struct {
	mu      sync.Mutex
	lease   time.Duration            // 手順の印の期限
	steps   map[string]*fakeStep     // 手順のキーごとの記録
	parents map[string]deploy.Parent // デプロイ ID ごとの親の投稿
}

func newFakeStore() *fakeStore {
	return &fakeStore{lease: dynamodb.DefaultLeaseTime, steps: map[string]*fakeStep{}, parents: map[string]deploy.Parent{}}
}

func (s *fakeStore) Claim(_ context.Context, key string, now time.Time) (dynamodb.ClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.steps[key]
	if !ok {
		st = &fakeStep{}
		s.steps[key] = st
	}
	if st.done {
		return dynamodb.ClaimDone, nil
	}
	if !st.leaseUntil.IsZero() && !st.leaseUntil.Before(now) {
		return dynamodb.ClaimBusy, nil
	}
	st.leaseUntil = now.Add(s.lease)
	return dynamodb.ClaimAcquired, nil
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

func (s *fakeStore) GetParent(_ context.Context, deploymentID string) (*deploy.Parent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.parents[deploymentID]
	if !ok {
		return nil, nil
	}
	return &p, nil
}

func (s *fakeStore) PutParent(_ context.Context, p *deploy.Parent, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.parents[p.DeploymentID]
	if p.Rev <= 1 && ok || p.Rev > 1 && (!ok || cur.Rev != p.Rev-1) {
		return dynamodb.ErrConflict
	}
	s.parents[p.DeploymentID] = *p
	return nil
}
