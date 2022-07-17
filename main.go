package main

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	gogpt "github.com/sashabaranov/go-gpt3"
)

const (
	redditURL          = "https://www.reddit.com"
	authRedditURL      = "https://oauth.reddit.com"
	redditAccUsername  = "Username"
	redditAccPassword  = "password"
	redditClientId     = "client_id"
	redditClientSecret = "client_secret"
	openaiApiKey       = "openai_api_key"
)

type TokenReq struct {
	AccessToken string `json:"access_token"`
}

type Messages struct {
	Kind string `json:"kind"`
	Data data   `json:"data"`
}

type data struct {
	After     string     `json:"after"`
	Dist      int        `json:"dist"`
	ModHash   string     `json:"modhash"`
	GeoFilter string     `json:"geo_filter"`
	Children  []children `json:"children"`
	Before    string     `json:"before"`
}

type children struct {
	Kind string       `json:"kind"`
	Data childrenData `json:"data"`
}

type childrenData struct {
	FirstMessage          string  `json:"first_message"`
	FirstMessageName      string  `json:"first_message_name"`
	Subreddit             string  `json:"subreddit"`
	Likes                 string  `json:"likes"`
	Replies               string  `json:"replies"`
	AuthorFullname        string  `json:"author_fullname"`
	Id                    string  `json:"id"`
	Subject               string  `json:"subject"`
	AssociatedAwardingId  string  `json:"associated_awarding_id"`
	Score                 int64   `json:"score"`
	Author                string  `json:"author"`
	NumComments           int64   `json:"num_comments"`
	ParentId              string  `json:"parent_id"`
	SubredditNamePrefixed string  `json:"subreddit_name_prefixed"`
	New                   bool    `json:"new"`
	Type                  string  `json:"type"`
	Body                  string  `json:"body"`
	LinkTitle             string  `json:"link_title"`
	Dest                  string  `json:"dest"`
	WasComment            bool    `json:"was_comment"`
	BodyHtml              string  `json:"body_html"`
	Name                  string  `json:"name"`
	Created               float64 `json:"created"`
	CreatedUTC            float64 `json:"created_utc"`
	Context               string  `json:"context"`
	Distinguished         string  `json:"distinguished"`
}

func main() {
	token := ""
	startTime := time.Now()
	var lastMsgs []string
	c := gogpt.NewClient(openaiApiKey)

	s := gocron.NewScheduler(time.UTC)

	s.Every(86400).Seconds().Do(func() {
		token = getToken()
		fmt.Println("Retrieved token:", token)
	})
	s.Every(10).Seconds().Do(func() {
		data := getData(token)

		for _, v := range data.Data.Children {
			post := v.Data
			if post.CreatedUTC > float64(startTime.Unix()) {
				if contains(lastMsgs, post.Id) {
				} else {
					m := ask(c, post.Body)
					sendReply(m, post.Id, token)
					fmt.Println("Replied: ", post.Id, m)
					lastMsgs = append(lastMsgs, post.Id)
				}
			}
		}
	})

	fmt.Println("Starting loop...")
	s.StartBlocking()
}

func getToken() string {
	bodyReq := []byte("grant_type=password&username=" + redditAccUsername + "&password=" + redditAccPassword)

	resp, _ := http.NewRequest("POST", redditURL+"/api/v1/access_token", bytes.NewBuffer(bodyReq))

	resp.Header.Add("Authorization", "Basic "+b64.URLEncoding.EncodeToString([]byte(redditClientId+":"+redditClientSecret)))
	client := &http.Client{}

	res, err := client.Do(resp)
	if err != nil {
		fmt.Println(err)
	}

	body, _ := io.ReadAll(res.Body)
	res.Body.Close()

	var tokenReq TokenReq

	json.Unmarshal(body, &tokenReq)
	return tokenReq.AccessToken
}

func getData(token string) Messages {
	resp, _ := http.NewRequest("GET", authRedditURL+"/message/inbox.json", nil)
	resp.Header.Add("Authorization", "bearer "+token)
	client := &http.Client{}

	res, err := client.Do(resp)
	if err != nil {
		fmt.Println(err)
	}

	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var messages Messages

	json.Unmarshal(body, &messages)
	return messages
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func ask(c *gogpt.Client, question string) string {
	ctx := context.Background()
	q := strings.ReplaceAll(question, "u/"+redditAccUsername+" ", "")
	req := gogpt.CompletionRequest{
		MaxTokens:        600,
		Prompt:           redditAccUsername + ` is an philosopher. \n\nHuman: ` + q + `\n` + redditAccUsername + `:`,
		Stop:             []string{"Human", "Human:", redditAccUsername, redditAccUsername + ":"},
		BestOf:           1,
		Temperature:      0.9,
		TopP:             0.75,
		FrequencyPenalty: 2,
		PresencePenalty:  1.7,
	}
	resp, err := c.CreateCompletion(ctx, "text-davinci-001", req)
	if err != nil {
		fmt.Println(err)
	}
	return resp.Choices[0].Text
}

func sendReply(message string, postId string, token string) {
	bodyReq := []byte("text=" + message + "&thing_id=t1_" + postId)
	resp, _ := http.NewRequest("POST", authRedditURL+"/api/comment", bytes.NewBuffer(bodyReq))

	resp.Header.Add("Authorization", "bearer "+token)
	client := &http.Client{}

	client.Do(resp)
}
