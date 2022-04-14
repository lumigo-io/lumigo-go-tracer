[![CircleCI](https://circleci.com/gh/lumigo-io/lumigo-go-tracer/tree/master.svg?style=svg&circle-token=57743ad287dcba62c92f315898af5d20ac3f1569)](https://circleci.com/gh/lumigo-io/lumigo-go-tracer/tree/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/lumigo-io/lumigo-go-tracer)](https://goreportcard.com/report/github.com/lumigo-io/lumigo-go-tracer)
[![codecov](https://codecov.io/gh/lumigo-io/lumigo-go-tracer/branch/master/graph/badge.svg?token=BOtQ9Myp1t)](https://codecov.io/gh/lumigo-io/lumigo-go-tracer)
[![GoDoc](https://godoc.org/github.com/lumigo-io/lumigo-go-tracer?status.svg)](https://godoc.org/github.com/lumigo-io/lumigo-go-tracer)

# go-tracer (BETA)

This is lumigo/lumigo-go-tracer, Lumigo's Golang agent for distributed tracing and performance monitoring.

## Installation

`lumigo-go-tracer` can be installed like any other Go library through `go get`:

```console
$ go get github.com/lumigo-io/lumigo-go-tracer
```

Or, if you are already using
[Go Modules](https://github.com/golang/go/wiki/Modules), you may specify a
version number as well:

```console
$ go get github.com/lumigo-io/lumigo-go-tracer@master
```

## Configuration
Lumigo Go tracer offers several different configuration options. Pass these to the Lambda function as environment variables:


| Name                         | Type      | Description                 | Required          |
|------------------------------|-----------|-----------------------------|-------------------|
| LUMIGO_USE_TRACER_EXTENSION  | bool      | Enables usage of Go tracer  | true              |
| LUMIGO_DEBUG                 | bool      | Enables debug logging       | false             |

## Usage

You need a lumigo token which you can find under the `Project Settings` and `Tracing` tab in lumigo platform. Then you need just to wrap your Lambda:

```go
import (
  //other imports

  lumigotracer "github.com/lumigo-io/lumigo-go-tracer"
)

type MyEvent struct {
  Name string `json:"name"`
}

func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
  return fmt.Sprintf("Hello %s!", name.Name ), nil
}

func main() {
	wrappedHandler := lumigotracer.WrapHandler(HandleRequest, &lumigotracer.Config{
		Token:       "<your-token>",
	})
	lambda.Start(wrappedHandler)
}
```

For tracing AWS SDK v2.0 calls check the following example:

```go
  client := &http.Client{
    Transport: lumigotracer.NewTransport(http.DefaultTransport),
  }

  // for AWS SDK v1.x
  sess := session.Must(session.NewSession(&aws.Config{
    HTTPClient: client,
  }))

  svc := s3.New(sess)
  
  // for AWS SDK v2.x
  cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithHTTPClient(client))
	svc := s3.NewFromConfig(cfg)

```

For tracing HTTP calls check the following example:

```go
  client := &http.Client{
    Transport: lumigotracer.NewTransport(http.DefaultTransport),
  }
	req, _ := http.NewRequest("GET", "https://<your-url>", nil)

  // for net/http
	res, err := client.Do(req)

  // for golang.org/x/net/context/ctxhttp
	res, err := ctxhttp.Do(context.Background(), client, req)
```

In your lambda environment variables you need to set `LUMIGO_USE_TRACER_EXTENSION: true` and use the following layer for `us-east-1`: `arn:aws:lambda:us-east-1:114300393969:layer:lumigo-tracer-extension:36`. The layer will be available in more regions soon.

## Contributing
Contributions to this project are welcome from all! Below are a couple pointers on how to prepare your machine, as well as some information on testing.

### Required Tools:
- go v1.16 and later
- make

If you want to deploy the example lambda for real testing you need: 
- terraform 0.14.5

### Lint

Linting the codebase:
```
make lint
```

### Test suite

Run the test suite:
```
make test
```

### Check styles

Runs go vet and lint in parallel

```
make checks
```

### Deploy example

Deploys in AWS a lambda function wrapped by tracer and prints tracing in logs (stdout):

```
export AWS_PROFILE=<your-profile>
make deploy-example
```

After you finished testing just destroy the AWS infrastructure resources for Lambda:

```
export AWS_PROFILE=<your-profile>
make destroy-example
```

### Releases

Everytime we merge in master we push a new release version. Based on the semantic versioning
we use the follow format:

#### Patch Release

Example commit message:

`patch: fix a buf for spans`

#### Minor Release

Example commit message:

`minor: add a feature for tracking http`

#### Major Release

Example commit message:

`major: upgrade telemetry sdk`

After merging, a new tag will be pushed on the previous available version. IN parallel a Github Release will be pushed automatically. 

