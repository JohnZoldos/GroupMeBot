package main

import (
	"encoding/json"
	"fmt"
	"github.com/subosito/gotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	fmt.Println(url)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return body, err

}

func getAllMessages(groupId string, accessToken string) {
	beforeId := ""
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
		//fmt.Println(messageResponse)
		messagesBatch := messageResponse.MessagesMap.Messages
		lastMessage := messagesBatch[len(messagesBatch) - 1]
		fmt.Println(lastMessage)
		beforeId = lastMessage.MessageId

	}
}

func main() {
	gotenv.Load()
	accessToken := os.Getenv("ACCESS_TOKEN")
	groups := getGroups(accessToken)
	for _, group := range groups.Groups {
		if "7342563" == (group.GroupId) {
			fmt.Println(group.Name)
			getAllMessages(group.GroupId, accessToken)
		}
	}

}