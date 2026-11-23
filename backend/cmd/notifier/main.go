// notifier は、デプロイの EventBridge のイベントを受け、デプロイごとに 1 つの Slack のスレッドにまとめる Lambda
//   - 処理の流れは internal/notifier、テーブルのアイテムの置き方は internal/dynamodb にある
//   - Slack のボットトークンは、コールドスタートのときに 1 回だけ SSM パラメータストアから読む
//   - ECS のデプロイのページの URL は、ECS のイベントごとに ListServiceDeployments で引く (実行ロールに ecs:ListServiceDeployments が要る)
//
// 環境変数:
//   - TABLE_NAME: 親の投稿と手順の記録を残す DynamoDB のテーブル (パーティションキーは文字列の pk、TTL は expires_at)
//   - SLACK_TOKEN_PARAMETER: ボットトークンを入れた SSM パラメータストアの SecureString の名前
//   - SLACK_CHANNEL_ID: 投稿するチャンネルの ID
//   - SERVICE_NAME: 通知するサービスの名前 (Deployment Started はサービスの名前を持たないため)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/tamaco489/amber/backend/internal/dynamodb"
	"github.com/tamaco489/amber/backend/internal/ecs"
	"github.com/tamaco489/amber/backend/internal/notifier"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// env は、Lambda の環境変数 (パッケージのコメントを参照)
type env struct {
	tableName      string // TABLE_NAME
	tokenParameter string // SLACK_TOKEN_PARAMETER
	channelID      string // SLACK_CHANNEL_ID
	serviceName    string // SERVICE_NAME
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	n, err := setup(context.Background())
	if err != nil {
		slog.Error("failed to set up", "error", err)
		os.Exit(1)
	}
	lambda.Start(n.Handle)
}

func loadEnv() (env, error) {
	e := env{
		tableName:      os.Getenv("TABLE_NAME"),
		tokenParameter: os.Getenv("SLACK_TOKEN_PARAMETER"),
		channelID:      os.Getenv("SLACK_CHANNEL_ID"),
		serviceName:    os.Getenv("SERVICE_NAME"),
	}

	var errs []error
	for _, v := range []struct{ name, value string }{
		{"TABLE_NAME", e.tableName},
		{"SLACK_TOKEN_PARAMETER", e.tokenParameter},
		{"SLACK_CHANNEL_ID", e.channelID},
		{"SERVICE_NAME", e.serviceName},
	} {
		if v.value == "" {
			errs = append(errs, fmt.Errorf("%s is not set", v.name))
		}
	}
	return e, errors.Join(errs...)
}

func setup(ctx context.Context) (*notifier.Notifier, error) {
	e, err := loadEnv()
	if err != nil {
		return nil, err
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	// DOC: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_GetParameter.html
	out, err := ssm.NewFromConfig(cfg).GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(e.tokenParameter),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get slack token: %w", err)
	}

	return &notifier.Notifier{
		Slack:   slack.NewClient(aws.ToString(out.Parameter.Value)),
		Store:   dynamodb.New(ddb.NewFromConfig(cfg), e.tableName),
		ECS:     ecs.New(awsecs.NewFromConfig(cfg)),
		Channel: e.channelID,
		Service: e.serviceName,
	}, nil
}
