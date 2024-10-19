package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAICall func (input string) string

func GetOpenAIClient(config config.Config) *openai.Client {
    openaiClient := openai.NewClient(config.OpenAISecret)

    resp, err := openaiClient.CreateChatCompletion(
        context.Background(),
	openai.ChatCompletionRequest{
	    Model: openai.GPT3Dot5Turbo, 
	    Messages: []openai.ChatCompletionMessage{
		{
		    Role: openai.ChatMessageRoleUser,
		    Content: "Hello",
		},
	    },
	},
    )

    if err != nil {
	fmt.Println(err.Error())
    }
    fmt.Println(resp.Choices[0].Message.Content)
    return openaiClient
}

func GetGeminiClient(config config.Config) *genai.Client {
    client, err := genai.NewClient(context.Background(), option.WithAPIKey(config.GeminiAISecret))
    if err != nil {
	log.Println(err.Error())
    }
    return client
}

func StartChatSession(client *genai.Client, startPrompt string, plan string) *genai.ChatSession {
    var model *genai.GenerativeModel
    if plan == Premium {
	model = client.GenerativeModel("gemini-1.5-pro")
    } else if plan == Basic {
	model = client.GenerativeModel("gemini-1.5-flash")
    } else {
	return nil
    }
    model.SetTemperature(0.6)
    year, month, day := time.Now().Date()
    now := fmt.Sprint(day, ".", month, ".", year)

    model.SystemInstruction = &genai.Content{
	Parts: []genai.Part{
		genai.Text(
		`All your responses must be in HTML and not in Markdown.
		You are a personal assistant and help with planning and organizing.
		Today is: ` + now + 
		`. The current calendar is:` + startPrompt + `
		. When you create a new event, inform the user of the event's start and end times.
		Additionally, insert text in the following format with the correct values so that the user can add the event with a single click:
		<button class="add-btn" id="title" onClick="addEventButton(title, start, end)">Add title to Calendar</button>
		Or this text when the user wants to remove an event
		<button class="add-btn" id="title"  onClick="removeEventButton(title, start, end)">Remove title from Calendar</button>
		Or this text when the user wants to move an event
		<button class="add-btn" id="title"  onClick="moveEventButton(title, start, end, movedStart, movedEnd)">Move title</button>
		Replace title, start, end, movedStart and movedEnd  with the corresponding strings. But do this ONLY when a new event is created.
		If no new event is to be created or removed, do not use this button.
		The button is for the user to approve the action so you can't do anything without his action.
		`,
	    ),
	},
    }

    chatSession := model.StartChat()
    return chatSession
}


func UpdateSchedule(session *genai.ChatSession, event *calendar.Event, action string) {
    session.History  = append(session.History, &genai.Content{
	Parts: []genai.Part{
	    genai.Text(
		`Die Aktion ` + action + `wurde auf Das Event: ` + event.Summary + `von ` + event.Start.DateTime + `bis ` + event.End.DateTime+ `angewandt`,
	    ),
	},
	Role: "user",
    })
}

func SendGeminiMessage(chatSession *genai.ChatSession, message string) (string, error) {
    resp, err := chatSession.SendMessage(context.Background(), genai.Text(message))
    if err != nil {
	log.Fatal(err.Error())
	return "", err
    }
    respString := ""
    for _, cand := range resp.Candidates {
	if cand.Content != nil {
	    for _, part := range cand.Content.Parts {
		respString += fmt.Sprintln(part)
	    }
	}
    }
    return respString, nil
}
