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

func StartChatSession(client *genai.Client, startPrompt string) *genai.ChatSession {
    model := client.GenerativeModel("gemini-1.5-flash")
    model.SetTemperature(0.6)
    year, month, day := time.Now().Date()
    now := fmt.Sprint(day, ".", month, ".", year)

    model.SystemInstruction = &genai.Content{
	Parts: []genai.Part{
		genai.Text(
		    `Alle deine Antworten müssen in HTML sein und nicht in Markdown.
		    Du bist ein persönlicher Assistent und hilfst dabei zu planen und zu organisieren.
		    Heute ist:` + now + 
		    `. Der aktuelle Kalender lautet:` +  startPrompt + `
		    . Wenn du ein neues Event erstellst, sage dem Nutzer die Start- und Endzeit des Events.
		    Füge zusätzlich einen Text in folgendem Format mit den korrekten Werten ein, damit der Nutzer das Event mit einem Klick hinzufügen kann:
		    <button class="add-btn" onClick="addEventButton(title, start, end)">Add title to Calendar</button>
		    Ersetze dabei title, start und end durch die entsprechenden Zeichenketten. Aber mache das NUR wenn ein neues Event erstellt wird.
		    Wenn kein Event Neu erstellt werden soll benutzt du diesen button nicht.
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
