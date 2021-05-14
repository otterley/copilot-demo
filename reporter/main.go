package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"

	"html/template"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type report struct {
	StationID string
	Observations []*observation
}

type observation struct {
	Timestamp string
	Temperature, WindSpeed string
}

const (
	// Seattle, WA, USA
	DefaultStation = "KSEA"

	StationEnvVar = "STATION_ID"
	DynamoDBTableNameEnvVar = "WEATHER_NAME"

	Template = `
	<html>
	<head>
	  <title>Weather report for {{ .StationID }}</title>
	<head>
	<body>
	<table>
	  <tr>
        <th>Timestamp</th><th>Temperature</th><th>Wind Speed</th>
	  </tr>
	{{ range .Observations }}
	  <tr>
	    <td>{{ .Timestamp }}</td><td>{{ .Temperature }}</td><td>{{ .WindSpeed }}</td>
	  </tr>
	{{ end }}
	</table>
	</body>
	</html>
`
)

var (
	dynamoDBTableName string
)

func init() {
	dynamoDBTableName = mustGetenv(DynamoDBTableNameEnvVar)
}

func getStationID() (string) {
	input := os.Getenv(StationEnvVar)
	if input == "" {
		return DefaultStation
	}
	return input
}

func mustGetenv(envName string) string {
	value := os.Getenv(envName)
	if value == "" {
		log.Fatalf("Missing or invalid environment variable %s", envName)
	}
	return value
}

func getObservations(stationID string) ([]*observation, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	ddbClient := dynamodb.NewFromConfig(cfg)

	response, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName: aws.String(dynamoDBTableName),
		KeyConditionExpression: aws.String("StationID = :stationID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":stationID": &types.AttributeValueMemberS{
				Value: stationID,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	observations := []*observation{}

	for _, item := range response.Items {
		o := &observation{}
		for name, value := range item {
			if name == "Timestamp" {
				o.Timestamp = value.(*types.AttributeValueMemberS).Value
			}
			if name == "WindSpeed" {
				o.WindSpeed = value.(*types.AttributeValueMemberN).Value
			}
			if name == "Temperature" {
				o.Temperature = value.(*types.AttributeValueMemberN).Value
			}
		}
		observations = append(observations, o)
	}
	return observations, nil
}


func main() {
	stationID := getStationID()

	tmpl, err := template.New("report").Parse(Template)
	if err != nil {
		log.Fatalf("%v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		observations, err := getObservations(stationID)
		if err != nil {
			log.Fatalf("%v", err)
		}
		err = tmpl.Execute(w, &report{
			StationID: stationID,
			Observations: observations,
		})
		if err != nil {
			log.Fatalf("%v", err)
		}
	})

	http.ListenAndServe(":8080", nil)
}

