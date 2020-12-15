// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command analyze performs sentiment, entity, entity sentiment, and syntax analysis
// on a string of text via the Cloud Natural Language API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	// [START imports]
	language "cloud.google.com/go/language/apiv1"
	"github.com/go-mail/mail"
	"github.com/golang/protobuf/proto"
	"github.com/jdkato/prose/v2"
	"google.golang.org/api/option"
	languagepb "google.golang.org/genproto/googleapis/cloud/language/v1"

	"github.com/citilinkru/camunda-client-go/processor"

	camundaclientgo "github.com/citilinkru/camunda-client-go"
)

type SantaData struct {
	Name               string `json:"name"`
	ParentEmailAddress string `json:"ParentEmailAddress"`
	Letter             string `json:"letter"`
}

type Gift []struct {
	Gifts      []string `json:"gift"`
	Types      []string `json:"type"`
	Sentiments []int    `json:"sentiment"`
	Amazon     []string `json:"amazon"`
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func santa(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "GET" {
		log.Println("GET Method Not Supported")
		http.Error(w, "GET Method not supported", 400)
	} else {
		fmt.Println("Got a POST!")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		log.Println(string(body))
		var t SantaData
		err = json.Unmarshal(body, &t)
		if err != nil {
			panic(err)
		}
		log.Println(t.Letter)
		w.WriteHeader(200)
		client := camundaclientgo.NewClient(camundaclientgo.ClientOptions{
			EndpointUrl: "http://davidgs.com:8080/engine-rest",
			ApiUser:     "demo",
			ApiPassword: "demo",
			Timeout:     time.Second * 10,
		})

		processKey := "santa"
		variables := map[string]camundaclientgo.Variable{
			"name":   {Value: t.Name, Type: "string"},
			"email":  {Value: t.ParentEmailAddress, Type: "string"},
			"letter": {Value: t.Letter, Type: "string"},
		}
		_, err = client.ProcessDefinition.StartInstance(
			camundaclientgo.QueryProcessDefinitionBy{Key: &processKey},
			camundaclientgo.ReqStartInstance{Variables: &variables},
		)
		if err != nil {
			log.Printf("Error starting process: %s\n", err)
			return
		}
		// aResult, _ := analyze(t.Letter)
		// searchAmazon(aResult)
	}
}

func searchAmazon(gResult camundaclientgo.Variable) (camundaclientgo.Variable, error) {
	var searches camundaclientgo.Variable
	Url, err := url.Parse("https://www.amazon.com")
	if err != nil {
		log.Fatal(err)
		return searches, err
	}
	Url.Path += "/s"


	var giftLookup Gift
	json.Unmarshal([]byte(fmt.Sprintf("%v", gResult.Value)), &giftLookup)
	for x := 0; x < len(giftLookup); x++ {
		giftLookup[x].Amazon = make([]string, len(giftLookup[x].Gifts))
		for y := 0; y < len(giftLookup[x].Gifts); y++ {
			parameters := url.Values{}
			// Url.RawQuery = parameters.Encode()
			fmt.Printf("Gift: %s\tType: %s\tSentiment: %d\n", giftLookup[x].Gifts[y], giftLookup[x].Types[y], giftLookup[x].Sentiments[y])
			parameters.Add("k", giftLookup[x].Gifts[y])
			Url.RawQuery = parameters.Encode()
			fmt.Println(Url)
			giftLookup[x].Amazon[y] = Url.String()
		}
	}
	bytes, err := json.Marshal(giftLookup)
	if err != nil {
		log.Fatal(err)
		return searches, err
	}
	var js string = "json"
	vInfo := camundaclientgo.ValueInfo{ObjectTypeName: &js, SerializationDataFormat: &js}

	searches.Value = string(bytes)
	searches.Type = "string"
	searches.ValueInfo = vInfo
	return searches, nil

}

func sendEmail(vars map[string]camundaclientgo.Variable) (bool, error) {
	var letterBody strings.Builder
	fmt.Fprintf(&letterBody, "<p>Seasons Greetings!</p>\n<p></p>\n<p>Guess what? %s has written me a letter asking for a few things. As I've now retired to a beach in Thailand, I thought maybe you'd like to know what %s asked for. Here's the letter:</p>\n<p></p>\n<blockquote><i>\"%s\"</i></blockquote>\n<p></p>\n", fmt.Sprintf("%v", vars["name"].Value), fmt.Sprintf("%v", vars["name"].Value), fmt.Sprintf("%v", vars["letter"].Value))

	fmt.Fprintf(&letterBody, "<p>I've taken the liberty of figuring out which things they want most, and provided you with a list so that you can just purchase these items directly. I know, it's put the elves out of work, but they're a resourceful lot and will undoubtedly figure out something to do with themselves. And no, they are not available for purchase.</p>\n<p>So, that list:</p>\n<p></p>\n<od>\n")
	var giftLookup Gift
	json.Unmarshal([]byte(fmt.Sprintf("%v", vars["links"].Value)), &giftLookup)
	for x := 0; x < len(giftLookup); x++ {
		for y := 0; y < len(giftLookup[x].Gifts); y++ {
			if giftLookup[x].Gifts[y] == "" {
				continue
			} else {
			fmt.Fprintf(&letterBody, "<li> <a href=\"%s\">%s</a> ", giftLookup[x].Amazon[y], giftLookup[x].Gifts[y])
			if giftLookup[x].Sentiments[y] < 0 {
				letterBody.WriteString(" ⚠️</li>\n")
			} else if giftLookup[x].Sentiments[y] == 0 {
				letterBody.WriteString(" ⁉️</li>\n")
			} else if giftLookup[x].Sentiments[y] > 0 && giftLookup[x].Sentiments[y] < 5 {
				letterBody.WriteString(" ❗️</li>\n")
			} else {
				letterBody.WriteString(" ‼️</li>\n")
			}
		}
	}
	}
	fmt.Fprintf(&letterBody, "</ol>\n<p></p>\n<p>In case you're wondering, since I'm retired, I'm also lazy. So I've used some Artificial Intelligence (which really isn't all that intelligent) to sort of 'rate' what they asked for. I <i>could</i> have ordered the list, but as I just told you, I'm retired, and lazy. Here's the rating system:</p><p><blockquote><ul><li>⚠️: meh.</li><li>⁉️: Ok, I guess.</li><li>❗: Now we're talkin!</li><li>‼️: Oh please! Oh Please! Oh please!</li></ul></blockquote><p></p><p>All the best from me and Mrs. Claus</p>\n--\n<p>PS: Please don't write back to this email address. I'm retired!</p><p><a href=\"https://write-a-letter-to-santa.org/\">Write your own letter!</a>\n")
	emailAddr := vars["email"].Value
	fmt.Println("To: : ", fmt.Sprintf("%v", emailAddr))
	fmt.Println("Try sending mail...")

	d := mail.Dialer{Host: "www.write-a-letter-to-santa.org", Port: 25, Username: "santa", Password: "Toby66.Mime!"}
	m := mail.NewMessage()
	m.SetHeader("From", m.FormatAddress("no-reply@write-a-letter-to-santa.org", "Santa Claus"))
	m.SetHeader("To", fmt.Sprintf("%v", emailAddr))
	m.SetHeader("Subject", "A Letter from Santa")
	m.SetBody("text/html", letterBody.String())
	// m.SetBody("text/plain", letterBody.String())
	if err := d.DialAndSend(m); err != nil {
		return false, err
	}
	return true, nil

}

func main() {
	fmt.Println("Starting up ... ")
	client := camundaclientgo.NewClient(camundaclientgo.ClientOptions{
		EndpointUrl: "http://davidgs.com:8080/engine-rest",
		// ApiUser:     "demo",
		// ApiPassword: "demo",
		Timeout: time.Second * 10,
	})
	logger := func(err error) {
		fmt.Println(err.Error())
	}
	proc := processor.NewProcessor(client, &processor.ProcessorOptions{
		WorkerId:                  "nlpProcessor",
		LockDuration:              time.Second * 5,
		MaxTasks:                  10,
		MaxParallelTaskPerHandler: 100,
		LongPollingTimeout:        5 * time.Second,
	}, logger)
	// NLP Handler
	proc.AddHandler(
		&[]camundaclientgo.QueryFetchAndLockTopic{
			{TopicName: "nlp-extraction"},
		},
		func(ctx *processor.Context) error {
			fmt.Printf("Running task %s. WorkerId: %s. TopicName: %s\n", ctx.Task.Id, ctx.Task.WorkerId, ctx.Task.TopicName)
			var sentRes camundaclientgo.Variable
			var err error
			var nlp bool
			varb := ctx.Task.Variables
			text := fmt.Sprintf("%v", varb["letter"].Value)
			fmt.Println(text)
			sentRes, nlp, err = analyze(text)
			if err != nil {
				log.Fatal(err)
			}
			vars := make(map[string]camundaclientgo.Variable)
			vars["status"] = camundaclientgo.Variable{Value: "true", Type: "boolean"}
			vars["nlp"] = camundaclientgo.Variable{Value: strconv.FormatBool(nlp), Type: "boolean"}
			vars["gifts"] = sentRes
			err = ctx.Complete(processor.QueryComplete{
				Variables: &vars,
			})
			if err != nil {
				fmt.Printf("Error set complete task %s: %s\n", ctx.Task.Id, err)
			}

			fmt.Printf("Task %s completed\n", ctx.Task.Id)
			return nil
		},
	)
	// Amazon search handler
	proc.AddHandler(
		&[]camundaclientgo.QueryFetchAndLockTopic{
			{TopicName: "amazon-search"},
		},
		func(ctx *processor.Context) error {
			fmt.Printf("Running task %s. WorkerId: %s. TopicName: %s\n", ctx.Task.Id, ctx.Task.WorkerId, ctx.Task.TopicName)

			links, err := searchAmazon(ctx.Task.Variables["gifts"])
			vars := make(map[string]camundaclientgo.Variable)
			vars["status"] = camundaclientgo.Variable{Value: "true", Type: "boolean"}
			vars["links"] = links
			err = ctx.Complete(processor.QueryComplete{
				Variables: &vars,
			})
			if err != nil {
				fmt.Printf("Error set complete task %s: %s\n", ctx.Task.Id, err)
			}

			fmt.Printf("Task %s completed\n", ctx.Task.Id)
			return nil
		},
	)
	// sendEmail
	proc.AddHandler(
		&[]camundaclientgo.QueryFetchAndLockTopic{
			{TopicName: "send-email"},
		},
		func(ctx *processor.Context) error {
			fmt.Printf("Running task %s. WorkerId: %s. TopicName: %s\n", ctx.Task.Id, ctx.Task.WorkerId, ctx.Task.TopicName)

			success, err := sendEmail(ctx.Task.Variables)
			err = ctx.Complete(processor.QueryComplete{
				Variables: &map[string]camundaclientgo.Variable{
					"status": {Value: strconv.FormatBool(success), Type: "boolean"},
				},
			})
			if err != nil {
				fmt.Printf("Error set complete task %s: %s\n", ctx.Task.Id, err)
			}

			fmt.Printf("Task %s completed\n", ctx.Task.Id)
			return nil
		},
	)
	fmt.Println("Done setting up Proc.")

	http.HandleFunc("/santa", santa)
err := http.ListenAndServeTLS(":9091", "/home/santa/ssl/certs/write_a_letter_to_santa_org_c1658_d7935_1639094399_401612a37d185ba05e8f3444da857270.crt", "/home/santa/ssl/keys/c1658_d7935_22cc5119c03ebb38ab5995713e2e2d21.key", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func analyze(letter string) (camundaclientgo.Variable, bool, error) {
	var cClient camundaclientgo.Variable
        var nlp = false;
	ctx := context.Background()
	client, err := language.NewClient(ctx, option.WithCredentialsFile("credentials.json"))
	if err != nil {
		log.Fatal(err)
		return cClient, nlp, err
	}

	doc, _ := prose.NewDocument(letter)
	sents := doc.Sentences()
	fmt.Printf("Letter is %d sentences long.\n", len(sents)) // 2

	var js string = "json"
	vInfo := camundaclientgo.ValueInfo{ObjectTypeName: &js, SerializationDataFormat: &js}
	var gifts Gift = make(Gift, len(sents))

	x := 0
	for _, sent := range sents {
		fmt.Printf("Sentence: %s\n", sent.Text)
		sentiment, err := analyzeSentiment(ctx, client, sent.Text)
		if err != nil {
			log.Fatal(err)
			return cClient, false, err
		}
		if sentiment.DocumentSentiment.Score >= 0 {
			fmt.Printf("Sentiment: %1f, positive\t", sentiment.DocumentSentiment.Score)
		} else {
			fmt.Printf("Sentiment: %1f negative\t", sentiment.DocumentSentiment.Score)
		}

		entities, err := analyzeEntities(ctx, client, sent.Text)
		gifts[x].Gifts = make([]string, len(entities.Entities))
		gifts[x].Types = make([]string, len(entities.Entities))
		gifts[x].Sentiments = make([]int, len(entities.Entities))
		//gifts[x].Amazon = make([]string, len(entities.Entities))
		for y := 0; y < len(entities.Entities); y++ {
			t := entities.Entities[y].Type.String()
			if t == "PERSON" || t == "LOCATION" || t == "NUMBER" {
				continue
			} else {
			gifts[x].Gifts[y] = entities.Entities[y].Name
			gifts[x].Types[y] = entities.Entities[y].Type.String()
			gifts[x].Sentiments[y] = int(sentiment.DocumentSentiment.Score * 10)
			if entities.Entities[y].Type.String() == "CONSUMER_GOOD" {
                           nlp = true;
                        }

			fmt.Printf("Item: %s\t Type: %s\n", entities.Entities[y].Name, entities.Entities[y].Type)
			}
		}
		x++
	}

	bytes, err := json.Marshal(gifts)
	if err != nil {
		log.Fatal(err)
		return cClient, false,  err
	}
	cClient.Value = string(bytes)
	cClient.Type = "string"
	cClient.ValueInfo = vInfo
	return cClient, nlp, nil
}

func usage(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stderr, "usage: analyze [entities|sentiment|syntax|entitysentiment|classify] <text>")
	os.Exit(2)
}

// [START language_entities_text]

func analyzeEntities(ctx context.Context, client *language.Client, text string) (*languagepb.AnalyzeEntitiesResponse, error) {
	return client.AnalyzeEntities(ctx, &languagepb.AnalyzeEntitiesRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: text,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
		EncodingType: languagepb.EncodingType_UTF8,
	})
}

// [END language_entities_text]

// [START language_sentiment_text]

func analyzeSentiment(ctx context.Context, client *language.Client, text string) (*languagepb.AnalyzeSentimentResponse, error) {
	return client.AnalyzeSentiment(ctx, &languagepb.AnalyzeSentimentRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: text,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
	})
}

// [END language_sentiment_text]

// [START language_syntax_text]

func analyzeSyntax(ctx context.Context, client *language.Client, text string) (*languagepb.AnnotateTextResponse, error) {
	return client.AnnotateText(ctx, &languagepb.AnnotateTextRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: text,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
		Features: &languagepb.AnnotateTextRequest_Features{
			ExtractSyntax: true,
		},
		EncodingType: languagepb.EncodingType_UTF8,
	})
}

// [END language_syntax_text]

// [START language_classify_text]

func classifyText(ctx context.Context, client *language.Client, text string) (*languagepb.ClassifyTextResponse, error) {
	return client.ClassifyText(ctx, &languagepb.ClassifyTextRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: text,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
	})
}

// [END language_classify_text]

func printResp(v proto.Message, err error) {
	if err != nil {
		log.Fatal(err)
	}
	proto.MarshalText(os.Stdout, v)
}
