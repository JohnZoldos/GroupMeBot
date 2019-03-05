package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/subosito/gotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

const urlBase  = "https://api.groupme.com/v3"

type Group struct {
	Id		string	`json:"id"`
	GroupId	string	`json:"group_id"`
	Name	string	`json:"name"`
}


type Groups struct{
	Groups	[]Group	`json:"response"`
}

type Message struct {
	Name		string		`json:"name"`
	Text		string		`json:"text"`
	MessageId	string		`json:"id"`
	FavoriteBy	[]string	`json:"favorited_by"`
	TimeSent    int64       `json:"created_at"`
}

type Messages struct {
	Messages []Message `json:"messages"`
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

func addMessagesFromDate(year int, month time.Month, day int, messages []Message, messagesFromDate* []Message) {
	for _, message := range(messages) {
		messageDate := time.Unix(message.TimeSent, 0)
		messageYear, messageMonth, messageDay := messageDate.Date()
		if messageYear == year {
			continue
		}
		if messageMonth == month && messageDay == day {
			*messagesFromDate = append(*messagesFromDate, message)
			fmt.Println("Hey we added one")
		}
	}

}

func getAllMessages(groupId string, accessToken string, date time.Time) []Message{
	year, month, day := date.Date()

	beforeId := ""
	var messagesFromDate []Message
	for {
		body, err := getMessageBatch(groupId, accessToken, beforeId)
		if err != nil || len(body) == 0{
			break
		}
		messageResponse := MessagesResponse{}
		err = json.Unmarshal(body, &messageResponse)
		if err != nil {
			panic(err)
		}
		messagesBatch := messageResponse.MessagesMap.Messages
		addMessagesFromDate(year, month, day, messagesBatch, &messagesFromDate)
		lastMessage := messagesBatch[len(messagesBatch) - 1]
		//fmt.Println(lastMessage)
		beforeId = lastMessage.MessageId

	}
	return messagesFromDate
}

func getFavoriteMessage(messages* []Message) Message{
	var mostFavorited Message
	mostFavorites := -1
	for _, message := range(*messages) {
		favorites := len(message.FavoriteBy)
		if favorites > mostFavorites {
			mostFavorites = favorites
			mostFavorited = message
		}
	}
	return mostFavorited
}

func postMessage(message Message, accessToken string, botId string) {
	url := fmt.Sprintf("%s/bots/post", urlBase)
	params := map[string]interface{}{
		"bot_id": botId,
		"text":  message.Text,
	}
	bytesRepresentation, err := json.Marshal(params)
	fmt.Println(url)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

}

func main() {
	gotenv.Load()
	accessToken := os.Getenv("ACCESS_TOKEN")
	botId := os.Getenv("BOT_ID")

	groups := getGroups(accessToken)
	loc, _ := time.LoadLocation("EST")
	currentTime := time.Date(2019, 2, 23,1, 1, 1,1, loc)//time.Now()
	var messagesFromToday []Message
	for _, group := range groups.Groups {
		if "7342563" == (group.GroupId) {
			fmt.Println(group.Name)
			messagesFromToday = getAllMessages(group.GroupId, accessToken, currentTime)
		}
	}
	mostFavoritedMessage := getFavoriteMessage(&messagesFromToday)
	postMessage(mostFavoritedMessage, accessToken, botId)


}

//ensure timezone is set correctly, if this were running on an EC2 you might need to set the timezone manually