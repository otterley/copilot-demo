package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/buger/jsonparser"
	"github.com/go-resty/resty/v2"
)

type observation struct {
	stationID string
	timestamp string
	temperature, windSpeed *float64
}

const (
	// Seattle, WA, USA
	DefaultStation = "KSEA"

	StationEnvVar = "STATION_ID"
	DynamoDBTableNameEnvVar = "WEATHER_NAME"


	ObservationEndpoint = "https://api.weather.gov/stations/%s/observations/latest"
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

func getLatestObservation(stationID string) (o *observation, err error) {
	client := resty.New()
	response, err := client.R().Get(fmt.Sprintf(ObservationEndpoint, stationID))
	if err != nil {
		return nil, err
	}
	body := response.Body()

	o = &observation {
		stationID: stationID,
	}
	if timestamp, err := jsonparser.GetString(body, "properties", "timestamp"); err == nil {
		o.timestamp = timestamp
	}
	if temperature, err := jsonparser.GetFloat(body, "properties", "temperature", "value"); err == nil {
		o.temperature = &temperature
	}
	if windSpeed, err := jsonparser.GetFloat(body, "properties", "windSpeed", "value"); err == nil {
		o.windSpeed = &windSpeed
	}
	return o, err
}

func recordObservation(o *observation) error {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	item := map[string]types.AttributeValue{
		"StationID": &types.AttributeValueMemberS{
			Value: o.stationID,
		},
		"Timestamp": &types.AttributeValueMemberS{
			Value: o.timestamp,
		},
	}
	if o.temperature != nil {
		item["Temperature"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *o.temperature),
		}
	}
	if o.windSpeed != nil {
		item["WindSpeed"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *o.windSpeed),
		}
	}
	ddbClient := dynamodb.NewFromConfig(cfg)
	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dynamoDBTableName),
		Item: item,
	})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	stationID := getStationID()

	log.Printf("Collecting observation for %s\n", stationID)
	wx, err := getLatestObservation(stationID)
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Printf("Recording observation for %s\n", stationID)
	err = recordObservation(wx)
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Println("Observation successfully recorded.")
}