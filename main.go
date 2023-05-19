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

type BotInfo struct {
	ID    string
	Name  string
	BotID string
}

func FixString(input string) string {
	reg := regexp.MustCompile("<.*?> |\n")
	return strings.TrimPrefix(reg.ReplaceAllString(input, " "), " ")
}

func shouldSkipMessage(ev *slackevents.MessageEvent, me BotInfo) bool {
	if ev.SubType == "channel_topic" {
		return true
	}
	if strings.HasPrefix(ev.Text, "&gt;") {
		return true
	}

	// Regex pattern for Slack usernames.
	usernamePattern := regexp.MustCompile("<@U[A-Z0-9]+>")
	usernames := usernamePattern.FindAllString(ev.Text, -1)

	// If the message contains any usernames and none of them are the bot's username, skip the message.
	if len(usernames) > 0 {
		containsBotUsername := false
		containsOtherUsername := false
		for _, username := range usernames {
			if username == fmt.Sprintf("<@%s>", me.ID) {
				containsBotUsername = true
			} else {
				containsOtherUsername = true
			}
		}

		if !containsBotUsername || containsOtherUsername {
			return true
		}
	}

	// Regex pattern for URLs.
	urlPattern := regexp.MustCompile(`http[s]?://(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*\\(\\),]|(?:%[0-9a-fA-F][0-9a-fA-F]))+`)
	urls := urlPattern.FindAllString(ev.Text, -1)

	// If the message contains any URLs, skip the message.
	if len(urls) > 0 {
		return true
	}

	return false
}

func main() {
	var s Specification
	err := envconfig.Process("slackse", &s)
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Println(FixString("<uahsd> pikker\nhund"))

	megahal := exec.Command("./megahal")
	stdin, err := megahal.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := megahal.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := megahal.Start(); err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(stdout)

	megahalIn := make(chan string, 128)

	api := slack.New(s.BotToken, slack.OptionDebug(false),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(s.AppToken))

	users, err := api.GetUsers()
	if err != nil {
		panic(err)
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
						if ev.SubType == "channel_topic" {
							fmt.Println("This was a topic change")
							break
						}
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
				_, err := stdin.Write([]byte(x))
				if err != nil {
					panic(err)
				}
			case <-time.After(30 * time.Second):
				_, err := stdin.Write([]byte("#SAVE\n"))
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	go func() {
		for scanner.Scan() {
			msg := strings.SplitN(scanner.Text(), " - ", 2)
			if len(msg) == 2 {
				fmt.Printf("Stdout: '%s'\n", msg[1])
				_, _, err := api.PostMessage(s.ChatChannel, slack.MsgOptionText(msg[1], false))
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
