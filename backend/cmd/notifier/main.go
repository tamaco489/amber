// notifier は、デプロイの EventBridge のイベントを受け、デプロイごとに 1 つの Slack のスレッドにまとめる Lambda
//   - 処理の流れは internal/notifier、テーブルのアイテムの置き方は internal/dynamodb にある
//   - Slack のボットトークンと、起動した人にメンションするための対応表は、コールドスタートのときに 1 回だけ SSM パラメータストアから読む
//   - ECS のデプロイのページの URL は、ECS のイベントごとに ListServiceDeployments で引く (実行ロールに ecs:ListServiceDeployments が要る)
//
// 環境変数:
//   - TABLE_NAME: 親の投稿と手順の記録を残す DynamoDB のテーブル (パーティションキーは文字列の pk、TTL は expires_at)
//   - SLACK_TOKEN_PARAMETER: ボットトークンを入れた SSM パラメータストアの SecureString の名前
//   - SLACK_CHANNEL_ID: 投稿するチャンネルの ID
//   - SERVICE_NAME: 通知するサービスの名前 (Deployment Started はサービスの名前を持たないため)
//   - SLACK_USER_IDS_PARAMETER: GitHub のユーザー名から Slack のユーザー ID を引く対応表 (JSON) を入れた SSM パラメータストアの SecureString の名前
//     任意。無いときと、読めないときは、メンションせずに通知する
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
	tableName        string // TABLE_NAME
	tokenParameter   string // SLACK_TOKEN_PARAMETER
	channelID        string // SLACK_CHANNEL_ID
	serviceName      string // SERVICE_NAME
	userIDsParameter string // SLACK_USER_IDS_PARAMETER (任意)
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
		tableName:        os.Getenv("TABLE_NAME"),
		tokenParameter:   os.Getenv("SLACK_TOKEN_PARAMETER"),
		channelID:        os.Getenv("SLACK_CHANNEL_ID"),
		serviceName:      os.Getenv("SERVICE_NAME"),
		userIDsParameter: os.Getenv("SLACK_USER_IDS_PARAMETER"),
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

	params := ssm.NewFromConfig(cfg)
	token, err := getParameter(ctx, params, e.tokenParameter)
	if err != nil {
		return nil, fmt.Errorf("get slack token: %w", err)
	}

	return &notifier.Notifier{
		Slack:   slack.NewClient(token),
		Store:   dynamodb.New(ddb.NewFromConfig(cfg), e.tableName),
		ECS:     ecs.New(awsecs.NewFromConfig(cfg)),
		Channel: e.channelID,
		Service: e.serviceName,
		UserIDs: loadUserIDs(ctx, params, e.userIDsParameter),
	}, nil
}

// getParameter は、SSM パラメータストアの SecureString を復号して読む
//
// DOC: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_GetParameter.html
func getParameter(ctx context.Context, client *ssm.Client, name string) (string, error) {
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Parameter.Value), nil
}

// loadUserIDs は、起動した人にメンションするための対応表を読む
//   - メンションは無くても通知は届くため、読めなくても止めない。ログに残し、nil (メンションしない) を返す
func loadUserIDs(ctx context.Context, client *ssm.Client, name string) slack.UserIDs {
	if name == "" {
		slog.WarnContext(ctx, "SLACK_USER_IDS_PARAMETER is not set; notify without mentions")
		return nil
	}

	data, err := getParameter(ctx, client, name)
	if err != nil {
		slog.WarnContext(ctx, "failed to get slack user ids; notify without mentions", "parameter", name, "error", err)
		return nil
	}

	ids, err := slack.ParseUserIDs(data)
	if err != nil {
		slog.WarnContext(ctx, "failed to parse slack user ids; notify without mentions", "parameter", name, "error", err)
		return nil
	}
	slog.InfoContext(ctx, "loaded slack user ids", "count", len(ids))
	return ids
}
