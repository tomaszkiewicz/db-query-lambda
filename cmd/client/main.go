package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/tomaszkiewicz/db-query-lambda/pkg/payload"
)

const (
	functionName = "db-query"
)

func main() {
	request := payload.QueryRequest{}

	switch len(os.Args) {
	case 2:
		request.Query = os.Args[1]
	case 3:
		request.Database = os.Args[1]
		request.Query = os.Args[2]
	default:
		fmt.Println("usage:")
		fmt.Println(os.Args[0], "[database name] <query>")
		os.Exit(1)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{})

	lambdaPayload, err := json.Marshal(request)
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.Invoke(&lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      lambdaPayload,
	})
	if err != nil {
		log.Fatal(err)
	}

	var res payload.QueryResponse

	err = json.Unmarshal(result.Payload, &res)
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(res.Rows, "", "  ")
	fmt.Println(string(b))
}
