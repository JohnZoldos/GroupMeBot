package main

import (
	"GroupMeChatBot/cloudwatchTrigger"
	"GroupMeChatBot/dbConnection"
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/subosito/gotenv"
)

const urlBase = "https://api.groupme.com/v3"
const botName = "MemsBot"
const aviLink = "https://i.groupme.com/1024x1024.png.415633b4d1264b85859f977673e8438c"
const location = "EST"
const callbackURL = "https://7cygninyze.execute-api.us-east-2.amazonaws.com/default/callback"

var local = false
var menu = false
var testGroupBotID string

//BotInfo struct
type BotInfo struct {
	BotID string `json:"bot_id"`
}

//Bot struct
type Bot struct {
	Info BotInfo `json:"bot"`
}

//BotCreationResponse struct
type BotCreationResponse struct {
	Response Bot `json:"response"`
}

//Group struct
type Group struct {
	ID      string        `json:"id"`
	GroupID string        `json:"group_id"`
	Name    string        `json:"name"`
	Members []interface{} `json:"members"`
}

func (group Group) getNumMembers() int {
	return len(group.Members)
}

//OneGroup struct
type OneGroup struct {
	Group Group `json:"response"`
}

//Groups struct
type Groups struct {
	Groups []Group `json:"response"`
}

//Event struct
type Event struct {
	Type string `json:"type"`
}

//Attachment struct
type Attachment struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

//Message struct
type Message struct {
	Name        string       `json:"name"`
	Text        string       `json:"text"`
	MessageID   string       `json:"id"`
	FavoriteBy  []string     `json:"favorited_by"`
	TimeSent    int64        `json:"created_at"`
	Event       Event        `json:"event"`
	Attachments []Attachment `json:"attachments"`
	SenderType  string       `json:"sender_type"`

	numMembersAtTime int
}

func (message Message) numLikes() int {
	return len(message.FavoriteBy)
}

func (message Message) percentageLikes() float32 {
	return float32(message.numLikes()) / float32(message.numMembersAtTime)
}

//Messages struct
type Messages struct {
	Messages []*Message `json:"messages"`
}

//MessagesResponse struct
type MessagesResponse struct {
	MessagesMap Messages `json:"response"`
}

func getPageOfGroups(accessToken string, page int) Groups {
	log.Print("Getting page of groups.")
	resp, err := http.Get(fmt.Sprintf("%s/groups?token=%s&page=%d", urlBase, accessToken, page))
	if err != nil {
		log.Print("Fatal error reached when getting page of groups.")
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	log.Print("Page of groups retrieved.")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Fatal error reached when reading page of groups.")
		log.Fatalln(err)
	}
	groups := Groups{}
	log.Print("Got " + string(len(groups.Groups)) + " groups when getting page of groups.")
	err = json.Unmarshal(body, &groups)
	if err != nil {
		log.Print("Fatal error reached when unmarshaling page of groups.")
		panic(err)
	}
	return groups
}

func getMessageBatch(groupID string, accessToken string, beforeID string) ([]byte, error) {
	numMessages := 100
	url := fmt.Sprintf("%s/groups/%s/messages?token=%s&limit=%d", urlBase, groupID, accessToken, numMessages)
	if beforeID != "" {
		url += fmt.Sprintf("&before_id=%s", beforeID)
	}
	resp, err := http.Get(url)
	if err != nil {
		log.Print("Fatal error reached when getting message batch.")
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Fatal error reached when reading message batch.")
		log.Fatalln(err)
	}
	return body, err

}

func countUsersAddedOrRemoved(str string) int {
	count := 1
	for _, c := range str {
		if c == ',' {
			count++
		}
	}
	if count == 1 && strings.Contains(str, " and ") {
		count++
	}
	return count
}

func addMessagesFromDate(numMembers *int, year int, month time.Month, day int, messages *[]*Message, popularMessagesFromDate *[]Message, popularMessagesFromDateAlreadyReposted *[]Message, repostedAlreadyMap map[string]int) {

	for _, message := range *messages {
		loc, _ := time.LoadLocation(location)
		messageDate := time.Unix(message.TimeSent, 0).In(loc)
		messageYear, messageMonth, messageDay := messageDate.Date()
		if messageYear == year { //messages from this year don't need to be examined
			continue
		}
		if messageMonth != month || messageDay != day { //messages not from this date don't need to be examined
			continue
		}
		if message.Name == "MemsBot" { //identify and extract messages that have already been reposted by memsbot
			if strings.HasPrefix(message.Text, "Last Mem's Context:\n- ") {
				continue
			}
			re, err := regexp.Compile(` \n\n- .* \| \d{1,2}/\d{1,2}/\d{2} \| ❤️x\d*`)

			if err != nil {
				log.Print(fmt.Sprintf("error trying to compile regexp"))
				continue
			}
			messageText := message.Text

			match := re.FindStringIndex(messageText)
			if match == nil {
				continue
			}
			attachmentText := ""
			if len(message.Attachments) > 0 {
				attachmentText += message.Attachments[0].URL
			}
			if match[0]-1 <= 1 {
				messageText = attachmentText
			} else {
				messageText = messageText[1:match[0]-1] + attachmentText
			}
			if repostedAlreadyMap[messageText] == 0 {
				repostedAlreadyMap[messageText] = messageYear //avoids replacing previous entries in the map
			} else {
				log.Print(fmt.Sprintf("Not adding this message to the repostedAlready map because it's already in there %s", messageText))
			}
		}
		attachementText := ""
		if len(message.Attachments) > 0 {
			attachementText += message.Attachments[0].URL
		}
		messageText := message.Text + attachementText
		alreadyReposted := false
		if repostedAlreadyMap[messageText] >= year-1 { //add messages already posted by memsbot to a separate list
			log.Print(fmt.Sprintf("This message isn't a candidate to be reposted because it was reposted last year: %s", messageText))
			continue
		} else if repostedAlreadyMap[messageText] > 0 {
			alreadyReposted = true
		}
		if strings.Contains(message.Event.Type, "bot") || message.SenderType == "bot" { //other groupme messages (like those from polls and calendar events) don't get reposted
			continue
		}
		if message.Name == "GroupMe" && strings.Contains(message.Text, "added") {
			*numMembers = *numMembers - countUsersAddedOrRemoved(message.Text)
		}
		if message.Name == "GroupMe" && strings.Contains(message.Text, "removed") {
			*numMembers = *numMembers + countUsersAddedOrRemoved(message.Text)
		}
		message.numMembersAtTime = *numMembers
		if message.isPopular() {
			if alreadyReposted {
				*popularMessagesFromDateAlreadyReposted = append(*popularMessagesFromDateAlreadyReposted, *message)
			} else {
				*popularMessagesFromDate = append(*popularMessagesFromDate, *message)
			}
			log.Print(fmt.Sprintf("Adding message to popular messages from today. Its time is %d", message.TimeSent))
			log.Print(fmt.Sprintf("Message's month is %d and its day is %d", messageMonth, messageDay))
			log.Print(fmt.Sprintf("Message has been reposted before: %t", alreadyReposted))

		}
	}

}

func getPopularMessagesFromDate(group Group, accessToken string, date time.Time) []Message {
	groupID := group.GroupID
	numMembers := group.getNumMembers()
	year, month, day := date.Date()

	beforeID := ""
	var allMessages []Message
	var popularMessagesFromDate []Message
	var popularMessagesFromDateAlreadyReposted []Message

	repostedAlreadyMap := make(map[string]int)

	for {
		body, err := getMessageBatch(groupID, accessToken, beforeID)
		if err != nil || len(body) == 0 {
			break
		}
		messageResponse := MessagesResponse{}
		err = json.Unmarshal(body, &messageResponse)
		if err != nil {
			log.Print("Fatal error reached when unmarshalling message batch.")
			panic(err)
		}
		messagesBatch := messageResponse.MessagesMap.Messages
		addMessagesFromDate(&numMembers, year, month, day, &messagesBatch, &popularMessagesFromDate, &popularMessagesFromDateAlreadyReposted, repostedAlreadyMap)
		if len(messagesBatch) == 0 {
			break
		}
		lastMessage := messagesBatch[len(messagesBatch)-1]
		beforeID = lastMessage.MessageID

		for _, message := range messagesBatch {
			allMessages = append(allMessages, *message)
		}
	}

	if len(popularMessagesFromDate) == 0 {
		popularMessagesFromDate = popularMessagesFromDateAlreadyReposted
	}
	return popularMessagesFromDate
}

func (message Message) isPopular() bool {
	if strings.Contains(strings.ToLower(message.Text), "like this") {
		return false
	}
	if message.numMembersAtTime <= 5 && (message.numLikes() < message.numMembersAtTime-1) {
		return false
	} else if message.numMembersAtTime >= 17 && message.numLikes() < 8 {
		return false
	} else if message.numMembersAtTime > 5 && message.numMembersAtTime < 17 && message.numLikes() < (4+(message.numMembersAtTime-5)/3) {
		return false
	}
	return true

}

func postMessage(message Message, botID string) {
	loc, _ := time.LoadLocation(location)
	messageDate := time.Unix(message.TimeSent, 0).In(loc)
	messageYear, messageMonth, messageDay := messageDate.Date()
	url := fmt.Sprintf("%s/bots/post", urlBase)
	messageText := fmt.Sprintf("\"%s\"", message.Text)
	if len(messageText) == 2 {
		messageText = ""
	}
	text := fmt.Sprintf("%s \n\n- %s | %d/%d/%d | ❤️x%d", messageText, message.Name, int(messageMonth), messageDay, messageYear%1000, message.numLikes())
	if text == "\"\"" {
		text = ""
	}
	params := map[string]interface{}{
		"bot_id": botID,
		"text":   text,
	}
	if len(message.Attachments) > 0 {
		params["picture_url"] = message.Attachments[0].URL
	}
	bytesRepresentation, err := json.Marshal(params)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Print("Fatal error reached when posting message.")
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	log.Print("Post message api request completed.")

}

func getMessageToPost(messages *[]Message) Message {
	if len(*messages) == 0 {
		return Message{}
	}
	sort.Slice(*messages, func(i, j int) bool {
		return (*messages)[i].percentageLikes() > (*messages)[j].percentageLikes()
	})

	var total float32 = 0.0
	for _, message := range *messages {
		total += message.percentageLikes()
	}
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	randNum := rng.Float32() * total
	for _, message := range *messages {
		randNum -= message.percentageLikes()
		if randNum <= 0 {
			return message
		}
	}

	return (*messages)[0]

}

func getAllGroups(accessToken string) []Group {
	var allGroups []Group
	for i := 1; ; i++ {
		page := getPageOfGroups(accessToken, i)
		if len(page.Groups) == 0 {
			break
		}
		allGroups = append(allGroups, page.Groups...)
	}
	return allGroups
}

func createBot(groupID, accessToken string) string {
	url := fmt.Sprintf("%s/bots?token=%s", urlBase, accessToken)
	params := map[string]interface{}{
		"bot": map[string]interface{}{
			"name":         botName,
			"group_id":     groupID,
			"avatar_url":   aviLink,
			"callback_url": callbackURL,
		},
	}
	bytesRepresentation, err := json.Marshal(params)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	bot := BotCreationResponse{}
	err = json.Unmarshal(body, &bot)
	if err != nil {
		log.Fatalln(err)
	}
	return bot.Response.Info.BotID

}

func deleteBot(botID, accessToken string) {
	url := fmt.Sprintf("%s/bots/destroy?token=%s", urlBase, accessToken)
	params := map[string]interface{}{
		"bot_id": botID,
	}
	bytesRepresentation, err := json.Marshal(params)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
}

func handler() {
	gotenv.Load()
	accessToken := os.Getenv("ACCESS_TOKEN")
	log.Print("Getting groups...")
	groups := getAllGroups(accessToken)
	log.Print(fmt.Sprintf("Got %d groups.", len(groups)))
	if menu {
		log.Print(fmt.Sprintf("Accessing the menu"))
		showMenu(groups, accessToken)
	} else if local {
		log.Print(fmt.Sprintf("Local run..."))
		sendMessages(accessToken)

	} else {
		log.Print(fmt.Sprintf("Sending messages..."))
		sendMessages(accessToken)
		cloudwatchTrigger.UpdateTrigger()
	}
}

func showMenu(groups []Group, accessToken string) {
	fmt.Println("Make a selection:")
	fmt.Println("[1] Add the bot to a group.")
	fmt.Println("[2] Remove the bot from a group.")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	selection := scanner.Text()
	if scanner.Err() != nil {
		fmt.Println(scanner.Err())
	}
	if selection == "1" {
		botCreationMenu(groups, accessToken)
	} else if selection == "2" {
		botDeletionMenu(groups, accessToken)
	}
}

func botCreationMenu(groups []Group, accessToken string) {
	fmt.Println("\n\nHere are all the groups you are a member of. Enter the number corresponding to the group you want to add a bot to: ")
	groupIndex := menuHelper(groups)
	groupID := groups[groupIndex].GroupID
	botID := dbConnection.GetBotForGroup(groupID)
	if botID == "" {
		botID = createBot(groupID, accessToken)
		dbConnection.AddBot(groupID, botID)
	} else {
		fmt.Println("That group already has this bot.")
	}
}

func botDeletionMenu(groups []Group, accessToken string) {
	fmt.Println("\n\nHere are all the groups you are a member of. Enter the number corresponding to the group you want to remove a bot from: ")
	groupIndex := menuHelper(groups)
	groupID := groups[groupIndex].GroupID
	botID := dbConnection.GetBotForGroup(groupID)
	if botID == "" {
		fmt.Println("That group doesn't have this bot.")
	} else {
		dbConnection.RemoveBot(groupID)
		deleteBot(botID, accessToken)
	}
}

func menuHelper(groups []Group) int {
	fmt.Println("-------------------------------------------------------------------------------------------------------------------")
	for i, group := range groups {
		fmt.Println(fmt.Sprintf("[%d] %s", i, group.Name))
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	selectedGroup := scanner.Text()
	if scanner.Err() != nil {
		fmt.Println(scanner.Err())
	}
	var groupIndex int
	if index, err := strconv.Atoi(selectedGroup); err == nil {
		groupIndex = index
	} else {
		fmt.Println(err)
	}
	return groupIndex
}

func sendMessages(accessToken string) {
	log.Print("Initiating...")
	item := dbConnection.Item{}
	loc, _ := time.LoadLocation(location)
	currentTime := time.Now().In(loc)
	hour, min, _ := currentTime.Clock()
	_, month, day := currentTime.Date()
	timeAndDate := fmt.Sprintf("Current time is %d:%d and the date is %d/%d", hour, min, month, day)
	log.Print(timeAndDate)

	log.Print(fmt.Sprintf("Location is set as: %s", currentTime.Location().String()))
	log.Print(fmt.Sprintf("Local is: %s", currentTime.Local().String()))

	allItemsFromDatabase := dbConnection.GetAllItems().Items
	log.Print(fmt.Sprintf("%d items found in database", len(allItemsFromDatabase)))

	findTestGroup(allItemsFromDatabase, accessToken)

	for _, i := range allItemsFromDatabase {
		err := dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			log.Print("Got error unmarshalling. System will exit.")
			log.Print(err.Error())
			os.Exit(1)
		}
		var popularMessagesFromToday []Message
		group := getGroup(item.GroupId, accessToken)
		log.Print(fmt.Sprintf("Got group with name %s and id %s.", group.Name, group.GroupID))
		popularMessagesFromToday = getPopularMessagesFromDate(group, accessToken, currentTime)
		log.Print(fmt.Sprintf("Found %d popular messages from today for group %s", len(popularMessagesFromToday), group.Name))
		messageToPost := getMessageToPost(&popularMessagesFromToday)
		if messageToPost.numLikes() > 0 { //checking to see if the message returned was a default message object or if its a real message
			log.Print(fmt.Sprintf("Posting message: '%s' by %s", messageToPost.Text, messageToPost.Name))
			if local {
				item.BotId = testGroupBotID
			}
			postMessage(messageToPost, item.BotId)
			if !local {
				dbConnection.UpdateLastMessageId(group.GroupID, messageToPost.MessageID)
			}
		}
	}
}

func findTestGroup(dbItems []map[string]*dynamodb.AttributeValue, accessToken string) {
	if !local {
		return
	}
	item := dbConnection.Item{}
	for _, i := range dbItems {
		err := dynamodbattribute.UnmarshalMap(i, &item)
		if err != nil {
			log.Print("Got error unmarshalling. System will exit.")
			log.Print(err.Error())
			os.Exit(1)
		}
		group := getGroup(item.GroupId, accessToken)
		if group.Name == "Test Group" {
			testGroupBotID = item.BotId
			log.Print(fmt.Sprintf("Found test group %s, making the test bot id %s", group.ID, item.BotId))
			return
		}
	}

}

func getGroup(groupID, accessToken string) Group {
	url := fmt.Sprintf("%s/groups/%s?token=%s", urlBase, groupID, accessToken)
	resp, err := http.Get(url)
	if err != nil {
		log.Print("Fatal error reached when getting group.")
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Fatal error reached when reading after getting group.")
		log.Fatalln(err)
	}
	group := OneGroup{}
	err = json.Unmarshal(body, &group)
	if err != nil {
		log.Print("Fatal error reached when unmarshaling group.")
		log.Fatalln(err)
		panic(err)
	}
	return group.Group
}

func main() {
	menuFlag := flag.Bool("menu", false, "boolean to bring up the menu. Takes highest priority of the flags.")
	localFlag := flag.Bool("local", false, "boolean to run locally (but not to bring up the menu)")

	flag.Parse()

	gotenv.Load()
	if *menuFlag {
		menu = true
		log.Print("Bringing up menu...")
		accessToken := os.Getenv("ACCESS_TOKEN")
		log.Print("Getting groups...")
		groups := getAllGroups(accessToken)
		log.Print(fmt.Sprintf("Got %d groups.", len(groups)))
		showMenu(groups, accessToken)
	} else if *localFlag {
		local = true
		log.Print(fmt.Sprintf("Running locally..."))
		handler()
	} else {
		log.Print("Running in prod...")
		lambda.Start(handler)
	}
}
