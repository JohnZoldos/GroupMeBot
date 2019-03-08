package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"os"
)


type Item struct {
	Group_id  string       `json:"group_id"`
	Bot_id	  string    `json:"bot_id"`
}

var dynamoClient *dynamodb.DynamoDB

const tableName = "GroupMeBot"

func startSession() {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		panic(err)
	}
	dynamoClient = dynamodb.New(session)

}

func AddBot(groupId string, botId string) {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}


	item := Item{
		Group_id: groupId,
		Bot_id: botId,
	}

	attributes, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		panic(err)
	}

	input := &dynamodb.PutItemInput{
		Item: attributes,
		TableName: aws.String(tableName),
	}

	_, err = dynamoClient.PutItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				fmt.Println(dynamodb.ErrCodeConditionalCheckFailedException, aerr.Error())
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
				fmt.Println(dynamodb.ErrCodeItemCollectionSizeLimitExceededException, aerr.Error())
			case dynamodb.ErrCodeTransactionConflictException:
				fmt.Println(dynamodb.ErrCodeTransactionConflictException, aerr.Error())
			case dynamodb.ErrCodeRequestLimitExceeded:
				fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return
	}
}


func GetBotForGroup(groupId string) string {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}
	//filt := expression.Name("group_id").Equal(expression.Value(groupId))


	// Get back the title, year, and rating
	proj := expression.NamesList(expression.Name("group_id"), expression.Name("bot_id"))

	expr, err := expression.NewBuilder().WithProjection(proj).Build()

	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}

	// Make the DynamoDB Query API call
	result, err := dynamoClient.Scan(params)
	item := Item{}
	for _, i := range result.Items {
		err = dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if item.Group_id == groupId {
			return item.Bot_id
		}
	}
	return ""

}

func GetAllBots(groupId string) string {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}
	//filt := expression.Name("group_id").Equal(expression.Value(groupId))


	// Get back the title, year, and rating
	proj := expression.NamesList(expression.Name("group_id"), expression.Name("bot_id"))

	expr, err := expression.NewBuilder().WithProjection(proj).Build()

	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}

	// Make the DynamoDB Query API call
	result, err := dynamoClient.Scan(params)
	item := Item{}
	for _, i := range result.Items {
		err = dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}
	return item.Bot_id

}



func removeBot(groupId string) {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"group_id": {
				S: aws.String(groupId),
			},
		},
		TableName: aws.String(tableName),
	}

	_, err := dynamoClient.DeleteItem(input)

	if err != nil {
		fmt.Println("Got error calling DeleteItem")
		fmt.Println(err.Error())
		return
	}
}
