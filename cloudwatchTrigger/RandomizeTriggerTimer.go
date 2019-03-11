package cloudwatchTrigger


import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"log"
	"math/rand"
	"os"
	"time"
)

func UpdateTrigger() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := cloudwatchevents.New(sess)

	source := rand.NewSource(time.Now().UnixNano())
	loc, _ := time.LoadLocation("UTC")
	rng := rand.New(source)
	randHour := int((rng.Float64()*10) + 13)
	randMinute := int(rng.Float64() * 60)
	dayOfWeek := int(time.Now().In(loc).Weekday())

	dayOfWeek += 2
	if dayOfWeek == 8 {
		dayOfWeek = 1
	}

	nextTrigger := cloudwatchevents.PutRuleInput{}
	nextTrigger.SetScheduleExpression(fmt.Sprintf("cron(%d %d ? * %d *)", randMinute, randHour, dayOfWeek))
	nextTrigger.SetName("DailyTrigger")
	_, err := svc.PutRule(&nextTrigger)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

}

