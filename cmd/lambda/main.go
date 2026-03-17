// Package main is the AWS Lambda entrypoint for ami-update-automation.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	amiresolver "github.com/kaiohenricunha/ami-update-automation/internal/ami"
	appconfig "github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/internal/handler"
	"github.com/kaiohenricunha/ami-update-automation/internal/logging"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/internal/secrets"
	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	logger := logging.NewLogger(os.Getenv("LOG_LEVEL"))
	logger.Info("starting ami-update-automation", slog.String("version", version))

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/var/task/config.yaml"
	}

	cfg, err := appconfig.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("failed to load AWS config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ssmClient := ssm.NewFromConfig(awsCfg)
	smClient := secretsmanager.NewFromConfig(awsCfg)

	apiURL := cfg.GitHub.APIURL
	h := handler.New(
		cfg,
		amiresolver.NewSSMResolver(ssmClient),
		scanner.NewRegistry(),
		vcs.NewGitHubProvider(apiURL),
		secrets.NewAWSSecretsManager(smClient),
		logger,
	)

	lambda.Start(func(ctx context.Context, _ map[string]any) (*types.HandlerResult, error) {
		return h.HandleEvent(ctx)
	})
}
