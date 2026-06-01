package config

import (
	"context"
	"encoding/base64"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

func NewFCMClient(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *messaging.Client {
	raw := config.String("FIREBASE_SERVICE_ACCOUNT_BASE64_JSON")
	if raw == "" {
		log.Fatal("Failed to get firebase service account from environment variable because it's empty")
	}

	creds, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		log.Fatal("Failed to decode base64 firebase service account because it's not valid base64-encoded", zap.Error(err))
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(creds))
	if err != nil {
		log.Fatal("Failed to initialize firebase app", zap.Error(err))
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		log.Fatal("Failed to get firebase messaging client", zap.Error(err))
	}

	return msgClient
}
