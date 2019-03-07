package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/subosito/gotenv"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
)

const urlBase  = "https://api.groupme.com/v3"

type Group struct {
	Id		string			`json:"id"`
	GroupId	string			`json:"group_id"`
	Name	string			`json:"name"`
	Members []interface{}   `json:"members"`
}

func (group Group) getNumMembers() int {
	return len(group.Members)
}


type Groups struct{
	Groups	[]Group	`json:"response"`
}

type Event struct {
	Type		string		`json:"type"`
}

type Attachment struct {
	Type 	string		`json:"type"`
	Url 	string		`json:"url"`
}


type Message struct {
	Name			 string			`json:"name"`
	Text			 string			`json:"text"`
	MessageId		 string			`json:"id"`
	FavoriteBy		 []string		`json:"favorited_by"`
	TimeSent    	 int64      	`json:"created_at"`
	Event			 Event			`json:"event"`
	Attachments 	[]Attachment	`json:"attachments"`

	numMembersAtTime int
}

func (message Message) numLikes() int {
	return len(message.FavoriteBy)
}

func (message Message) percentageLikes() float32 {
	return float32(message.numLikes())/float32(message.numMembersAtTime)
}

type Messages struct {
	Messages []*Message `json:"messages"`
}

type MessagesResponse struct {
	MessagesMap Messages `json:"response"`
}

func getGroups(accessToken string) Groups{
	resp, err := http.Get(urlBase + "/groups?token=" + accessToken)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	groups := Groups{}
	err = json.Unmarshal(body, &groups)
	if err != nil {
		panic(err)
	}
	return groups
}


func getMessageBatch(groupId string, accessToken string, before_id string) ([]byte, error) {
	numMessages := 100
	url := fmt.Sprintf("%s/groups/%s/messages?token=%s&limit=%d", urlBase, groupId, accessToken, numMessages)
	if before_id != "" {
		url += fmt.Sprintf("&before_id=%s", before_id)
	}
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
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

func addMessagesFromDate(numMembers* int, year int, month time.Month, day int, messages* []*Message, popularMessagesFromDate * []Message) {
	for _, message := range(*messages) {
		if strings.Contains(message.Event.Type, "bot") {
			continue
		}
		if message.Name == "GroupMe" && strings.Contains(message.Text, "added") {
			*numMembers = *numMembers - countUsersAddedOrRemoved(message.Text)
		}
		if message.Name == "GroupMe" && strings.Contains(message.Text, "removed") {
			*numMembers = *numMembers + countUsersAddedOrRemoved(message.Text)
		}
		message.numMembersAtTime = *numMembers
		messageDate := time.Unix(message.TimeSent, 0)
		messageYear, messageMonth, messageDay := messageDate.Date()
		if messageYear == year {
			continue
		}
		if messageMonth == month && messageDay == day && message.isPopular()  {
			*popularMessagesFromDate = append(*popularMessagesFromDate, *message)
		}
	}

}

func getPopularMessagesFromDate(group Group, accessToken string, date time.Time) []Message{
	groupId := group.GroupId
	numMembers := group.getNumMembers()
	year, month, day := date.Date()

	beforeId := ""
	var allMessages []Message
	var popularMessagesFromDate []Message
	for {
		body, err := getMessageBatch(groupId, accessToken, beforeId)
		if err != nil || len(body) == 0{
			break
		}
		//fmt.Println(string(body))
		messageResponse := MessagesResponse{}
		err = json.Unmarshal(body, &messageResponse)
		if err != nil {
			panic(err)
		}
		messagesBatch := messageResponse.MessagesMap.Messages
		addMessagesFromDate(&numMembers, year, month, day, &messagesBatch, &popularMessagesFromDate)
		lastMessage := messagesBatch[len(messagesBatch) - 1]
		//fmt.Println(lastMessage)
		beforeId = lastMessage.MessageId

		for _, message := range(messagesBatch) {
			allMessages = append(allMessages, *message)
		}
	}


	return popularMessagesFromDate
}

func (message Message) isPopular() bool{
	if strings.Contains(strings.ToLower(message.Text), "like this") {
		return false
	}
	if message.numMembersAtTime <= 5 && (message.numLikes() < message.numMembersAtTime - 1) {
		return false
	} else if message.numMembersAtTime >= 17 && message.numLikes() < 8 {
		return false
	} else if message.numMembersAtTime > 5 && message.numMembersAtTime < 17 && message.numLikes() < (4 + (message.numMembersAtTime - 5)/3) {
		return false
	}
	return true

}

func postMessage(message Message, accessToken string, botId string) {

	messageDate := time.Unix(message.TimeSent, 0)
	messageYear, messageMonth, messageDay := messageDate.Date()
	url := fmt.Sprintf("%s/bots/post", urlBase)
	text := fmt.Sprintf("\"%s\" \n\n- %s | %d/%d/%d | ❤️x%d", message.Text, message.Name, int(messageMonth), messageDay, messageYear%1000, message.numLikes())
	params := map[string]interface{}{
		"bot_id": botId,
		"text":  text,
	}
	if len(message.Attachments) > 0 {
		params["picture_url"] = message.Attachments[0].Url
	}
	bytesRepresentation, err := json.Marshal(params)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

}

func getMessageToPost(messages* []Message) Message {
	if len(*messages) == 0{
		return Message{}
	}
	sort.Slice(*messages, func(i, j int) bool {
		return (*messages)[i].percentageLikes() > (*messages)[j].percentageLikes()
	})

	var total float32 = 0.0
	for _, message := range(*messages) {
		total += message.percentageLikes()
	}
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	randNum := rng.Float32() * total
	for _, message := range(*messages){
		randNum -= message.percentageLikes()
		if randNum <= 0 {
			return message
		}
	}

	return (*messages)[0]

}

func main() {
	gotenv.Load()
	//accessToken := os.Getenv("ACCESS_TOKEN")
	//botId := os.Getenv("BOT_ID")
	//
	//groups := getGroups(accessToken)
	//for i:=1; i<31; i++ {
	//	loc, _ := time.LoadLocation("EST")
	//	currentTime := time.Date(2020, 11, i,1, 1, 1,1, loc)//time.Now()
	//	var popularMessagesFromToday []Message
	//	for _, group := range groups.Groups {
	//		if "7342563" == (group.GroupId) {
	//			fmt.Println(group.Name)
	//			popularMessagesFromToday = getPopularMessagesFromDate(group, accessToken, currentTime)
	//		}
	//	}
	//	messageToPost := getMessageToPost(&popularMessagesFromToday)
	//	if messageToPost.Text != "" {
	//		postMessage(messageToPost, accessToken, botId)
	//	}
	//}

	AddBot(1, "d")

}

//ensure timezone is set correctly, if this were running on an EC2 you might need to set the timezone manually