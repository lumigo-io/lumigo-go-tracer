package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	lumigotracer "github.com/lumigo-io/lumigo-go-tracer"
)

type MyEvent struct {
	Name string `json:"name"`
}

func HandleRequest(ctx context.Context, name MyEvent) (events.APIGatewayProxyResponse, error) {
	client := &http.Client{
		Transport: lumigotracer.NewTransport(http.DefaultTransport),
	}
	cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithHTTPClient(client))
	cfg.Region = "us-east-1"
	// testing S3
	svc := s3.NewFromConfig(cfg)
	_, err := svc.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		return events.APIGatewayProxyResponse{Body: "", StatusCode: 500}, err
	}

	svc.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String("test-bucket-go-tracer-2"),
	})

	// testing SSM
	ssmClient := ssm.NewFromConfig(cfg)
	input := &ssm.GetParameterInput{
		Name: aws.String("parameter-name"),
	}
	_, err = ssmClient.GetParameter(context.Background(), input)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: "ssm error", StatusCode: 500}, err
	}
	response := fmt.Sprintf("Hello %s!", name.Name)

	returnErr, ok := os.LookupEnv("RETURN_ERROR")
	if !ok {
		return events.APIGatewayProxyResponse{Body: response, StatusCode: 200}, nil
	}
	isReturnErr, err := strconv.ParseBool(returnErr)
	if err != nil {
		return events.APIGatewayProxyResponse{Body: response, StatusCode: 500}, err
	}
	// testing return error
	if isReturnErr {
		return events.APIGatewayProxyResponse{Body: response, StatusCode: 500}, errors.New("failed error")
	}
	// testing return
	return events.APIGatewayProxyResponse{Body: response, StatusCode: 200}, nil
}

func main() {
	os.Setenv("LUMIGO_DEBUG", "true")
	wrappedHandler := lumigotracer.WrapHandler(HandleRequest, &lumigotracer.Config{
		Token: "<insert your token>",
	})
	lambda.Start(wrappedHandler)
}
