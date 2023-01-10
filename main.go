package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/kelseyhightower/envconfig"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type Specification struct {
	AppToken    string
	BotToken    string
	ChatChannel string
}

func FixString(input string) string {
	reg := regexp.MustCompile("<.*?> |\n")
	return strings.TrimPrefix(reg.ReplaceAllString(input, " "), " ")
}

func main() {
	var s Specification
	err := envconfig.Process("slackse", &s)
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Println(FixString("<uahsd> pikker\nhund"))
	megahal := exec.Command("./megahal")

	f, err := pty.Start(megahal)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(f)

	megahalIn := make(chan string, 128)

	api := slack.New(s.BotToken, slack.OptionDebug(false),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(s.AppToken))

	users, err := api.GetUsers()
	if err != nil {
		panic(err)
	}

	type BotInfo struct {
		ID    string
		Name  string
		BotID string
	}
	var me BotInfo
	for _, user := range users {
		if user.Profile.ApiAppID != "" && user.Profile.BotID != "" && strings.Contains(s.AppToken, user.Profile.ApiAppID) {
			me.ID = user.ID
			me.Name = user.Name
			me.BotID = user.Profile.BotID
			break
		}
	}

	client := socketmode.New(api, socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)), socketmode.OptionDebug(false))

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				fmt.Println("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				fmt.Println("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				fmt.Printf("Event received: %+v\n", eventsAPIEvent)

				client.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.MemberJoinedChannelEvent:
						fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
					case *slackevents.MessageEvent:
						if strings.HasPrefix(ev.Text, "&gt;") {
							fmt.Println("This was a quote")
							break
						}
						if strings.HasPrefix(ev.Text, "&lt;") {
							fmt.Println("This was a username")
							break
						}
						if strings.Contains(ev.Text, fmt.Sprintf("<@%s>", me.ID)) {
							fmt.Println("This was a mention")
							msg := FixString(ev.Text)
							fmt.Println(msg)
							megahalIn <- msg + "\n"
							break
						}
						if ev.BotID == me.BotID {
							fmt.Println("The bot was here!")
							break
						}
						if ev.BotID != me.BotID {
							fmt.Println("Normal user, learning!")
							msg := FixString(ev.Text)
							fmt.Println(msg)
							megahalIn <- "#LEARN\n" + msg + "\n"
							break
						}
					}
				default:
					client.Debugf("unsupported Events API event received")
				}
			default:
				fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
			}
		}
	}()
	go func() {
		fmt.Println("started")
		for {
			select {
			case x := <-megahalIn:
				fmt.Println("Stdin: " + x)
				f.WriteString(x)
				//f.Sync()
			case <-time.After(30 * time.Second):
				f.WriteString("#SAVE\n")
				//f.Sync()
			}
		}
	}()

	go func() {
		for scanner.Scan() {
			msg := scanner.Text()
			if strings.HasPrefix(msg, "- ") {

				cleanMsg := strings.Replace(msg, "- ", "", 1)
				fmt.Println(cleanMsg)
				_, _, err := api.PostMessage(s.ChatChannel, slack.MsgOptionText(cleanMsg, false))
				if err != nil {
					fmt.Printf("failed posting message: %v", err)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}()

	client.Run()

}
