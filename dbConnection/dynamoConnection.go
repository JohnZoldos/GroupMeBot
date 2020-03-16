package dbConnection

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

type Item struct {
	GroupId       string `json:"group_id"`
	BotId         string `json:"bot_id"`
	LastMessageId string `json:"last_message_id"`
}

type ItemInfo struct {
	LastMessageId string `json:":l"`
}

type ItemKey struct {
	GroupId string `json:"group_id"`
}

var dynamoClient *dynamodb.DynamoDB

const tableName = "GroupMeBot"

func startSession() {
	log.Print("Dynamo session started.")
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		log.Print("Error reached when starting dynamo session")
		log.Print(err)
		panic(err)
	}
	dynamoClient = dynamodb.New(session)

}

func AddBot(groupId string, botId string) {
	if dynamoClient == nil {
		startSession() //should i shut it down manually? Optional, but recommended. Probably doesn't matter if using lambda?
	}

	item := Item{
		GroupId: groupId,
		BotId:   botId,
	}

	attributes, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		panic(err)
	}

	input := &dynamodb.PutItemInput{
		Item:      attributes,
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
	fmt.Println("Getting bot_id for group " + groupId)
	item := Item{}
	result := GetAllItems()
	for _, i := range result.Items {
		err := dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if item.GroupId == groupId {
			return item.BotId
		}
	}
	return ""

}

func GetAllItems() *dynamodb.ScanOutput {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}
	log.Print("Getting all items from db.")
	proj := expression.NamesList(expression.Name("group_id"), expression.Name("bot_id"))
	expr, _ := expression.NewBuilder().WithProjection(proj).Build()
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}
	// Make the DynamoDB Query API call
	result, err := dynamoClient.Scan(params)
	if err != nil {
		log.Print("Error reached when querying db. Exiting.")
		log.Print(err)
		//os.Exit(1)
	}

	log.Print("Got all items from db.")
	return result
}

func RemoveBot(groupId string) {
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

func UpdateLastMessageId(groupId, lastMessageId string) {
	if dynamoClient == nil {
		startSession() //should i shut it down manually?
	}
	info := ItemInfo{
		LastMessageId: lastMessageId,
	}

	item := ItemKey{
		GroupId: groupId,
	}

	update, err := dynamodbattribute.MarshalMap(info)
	if err != nil {
		fmt.Println("Got error marshalling info:")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	key, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		fmt.Println("Got error marshalling item:")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	input := &dynamodb.UpdateItemInput{
		Key:                       key,
		TableName:                 aws.String(tableName),
		UpdateExpression:          aws.String("set last_message_id = :l"),
		ExpressionAttributeValues: update,
	}

	_, err = dynamoClient.UpdateItem(input)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Println("Updated last message id completed!")

}
