package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"switchremote/back/internal/httpapi"
	"switchremote/back/internal/store"
	"switchremote/back/internal/tuya"
)

func main() {
	tuyaBaseURL := os.Getenv("TUYA_BASE_URL")
	if tuyaBaseURL == "" {
		tuyaBaseURL = "https://openapi.tuyaus.com"
	}
	db, err := store.Open(context.Background(), mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	handler := httpapi.New(
		db,
		tuya.New(tuyaBaseURL, mustEnv("TUYA_CLIENT_ID"), mustEnv("TUYA_CLIENT_SECRET")),
		[]byte(mustEnv("JWT_SECRET")),
	)

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
		return
	}
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing required environment variable %s", name)
	}
	return v
}
