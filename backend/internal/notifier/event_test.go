package notifier

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name            string
		raw             events.CloudWatchEvent
		wantKind        Kind
		wantService     string
		wantPushed      bool
		wantUnsupported bool
		wantErr         bool
	}{
		{name: "started", raw: startedEvent("1", true), wantKind: KindStarted, wantService: testService, wantPushed: true},
		{name: "started without push", raw: startedEvent("1", false), wantKind: KindStarted, wantService: testService},
		{name: "in progress", raw: evInProgress, wantKind: KindInProgress, wantService: "prd-amber-api"},
		{name: "rollback by circuit breaker", raw: evRollback, wantKind: KindRollback, wantService: "prd-amber-api"},
		{
			name:        "rollback by alarms",
			raw:         ecsEvent("1", ecsInProgress, "ECS deployment alarms: Rolling back to deploymentId ecs-svc/111."),
			wantKind:    KindRollback,
			wantService: "prd-amber-api",
		},
		{name: "completed", raw: evCompleted, wantKind: KindCompleted, wantService: "prd-amber-api"},
		{name: "failed", raw: evFailed, wantKind: KindFailed, wantService: "prd-amber-api"},
		{name: "unknown ecs event name", raw: ecsEvent("1", "SERVICE_DEPLOYMENT_STOPPED", ""), wantUnsupported: true},
		{name: "unknown source", raw: events.CloudWatchEvent{ID: "1", Source: "aws.ec2", DetailType: "x"}, wantUnsupported: true},
		{name: "no id", raw: events.CloudWatchEvent{Source: ecsSource, DetailType: ecsDetailType}, wantErr: true},
		{
			name:    "no deployment id",
			raw:     events.CloudWatchEvent{ID: "1", Source: ecsSource, DetailType: ecsDetailType, Detail: []byte(`{"eventName":"SERVICE_DEPLOYMENT_COMPLETED"}`)},
			wantErr: true,
		},
		{
			name:    "broken detail",
			raw:     events.CloudWatchEvent{ID: "1", Source: startedSource, DetailType: startedDetailType, Detail: []byte(`{`)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, err := ParseEvent(tt.raw, testService)

			if tt.wantUnsupported {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("err = %v, want ErrUnsupported", err)
				}
				return
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if errors.Is(err, ErrUnsupported) {
					t.Errorf("err = %v, want an error other than ErrUnsupported", err)
				}
				return
			}
			if ev.Kind != tt.wantKind {
				t.Errorf("kind = %s, want %s", ev.Kind, tt.wantKind)
			}
			if ev.Service != tt.wantService {
				t.Errorf("service = %q, want %q", ev.Service, tt.wantService)
			}
			if ev.Pushed != tt.wantPushed {
				t.Errorf("pushed = %v, want %v", ev.Pushed, tt.wantPushed)
			}
			if ev.DeploymentID != testDeployment {
				t.Errorf("deploymentId = %q, want %q", ev.DeploymentID, testDeployment)
			}
		})
	}
}

func TestFormatJST(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{name: "utc", in: time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC), want: "2026-11-23 11:12:30 (JST)"},
		{name: "crosses midnight", in: time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC), want: "2026-11-24 00:00:00 (JST)"},
		{name: "other zone", in: time.Date(2026, 11, 23, 0, 0, 0, 0, time.FixedZone("X", -5*60*60)), want: "2026-11-23 14:00:00 (JST)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatJST(tt.in); got != tt.want {
				t.Errorf("formatJST = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplyMessage(t *testing.T) {
	tests := []struct {
		name       string
		raw        events.CloudWatchEvent
		wantHeader string
		wantColor  string
		wantBody   []string
		notInBody  []string
	}{
		{
			name:       "started",
			raw:        startedEvent("1", true),
			wantHeader: ":rocket: Deploy started: v3",
			wantColor:  colorInfo,
			wantBody:   []string{"*Triggered by*\ntamaco489", "*Deployment ID*\n" + testDeployment, "*Task definition*\n" + testTaskDef, "*Run*\n" + testRunURL},
			notInBody:  []string{"Pushed"},
		},
		{
			name:       "redeploy of an existing image",
			raw:        startedEvent("1", false),
			wantHeader: ":rocket: Redeploy started: v3 (existing image)",
			wantColor:  colorInfo,
		},
		{
			name:       "in progress",
			raw:        evInProgress,
			wantHeader: ":hourglass_flowing_sand: Deployment in progress",
			wantColor:  colorWarning,
			wantBody:   []string{"*Service*\nprd-amber-api", "*Reason*\nECS deployment", "2026-11-23 11:12:30 (JST)"},
		},
		{name: "rollback", raw: evRollback, wantHeader: ":rewind: Rollback started", wantColor: colorDanger},
		{name: "completed", raw: evCompleted, wantHeader: ":white_check_mark: Deployment completed", wantColor: colorSuccess},
		{name: "failed", raw: evFailed, wantHeader: ":x: Deployment failed", wantColor: colorDanger, wantBody: []string{"task failed to start."}},
		{
			name:     "reason is escaped",
			raw:      ecsEvent("1", ecsFailed, "a <b> & c"),
			wantBody: []string{"a &lt;b&gt; &amp; c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, err := ParseEvent(tt.raw, testService)
			if err != nil {
				t.Fatal(err)
			}
			msg := replyMessage(ev, testChannel, "1700000000.000001")

			if msg.ThreadTS != "1700000000.000001" || msg.Channel != testChannel {
				t.Errorf("thread_ts = %q, channel = %q", msg.ThreadTS, msg.Channel)
			}
			if tt.wantHeader != "" && headerOf(msg) != tt.wantHeader {
				t.Errorf("header = %q, want %q", headerOf(msg), tt.wantHeader)
			}
			if tt.wantColor != "" && colorOf(msg) != tt.wantColor {
				t.Errorf("color = %q, want %q", colorOf(msg), tt.wantColor)
			}
			body := bodyOf(msg)
			for _, want := range tt.wantBody {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q:\n%s", want, body)
				}
			}
			for _, unwanted := range tt.notInBody {
				if strings.Contains(body, unwanted) {
					t.Errorf("body contains %q:\n%s", unwanted, body)
				}
			}
		})
	}
}
