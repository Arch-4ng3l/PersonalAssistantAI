package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAICall func(input string) string

func GetOpenAIClient(config config.Config) *openai.Client {
	openaiClient := openai.NewClient(config.OpenAISecret)

	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
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
		log.Println("NO PLAN")
		return nil
	}

	model.SetTemperature(0.7)

	year, month, day := time.Now().Date()
	now := fmt.Sprintf("%02d.%02d.%d", day, month, year)
	model.ResponseMIMEType = "application/json"

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(
				`You are a professional calendar management assistant with direct access to modify the calendar. Your responses should always be in valid JSON format.

Current date: ` + now + `
Current calendar events: ` + startPrompt + `

Guidelines for interactions:
1. Always respond with a JSON object containing:
   {
     "understood": boolean,     // Whether you understood the request
     "action": string,         // The action being taken (e.g. "add_event", "remove_event", "reschedule", "info")
     "details": {              // Details of the action
       "title": string,        // Event title if applicable
       "startTime": string,    // Start time if applicable
       "endTime": string,      // End time if applicable
       "originalStart": string, // For reschedule - original start time
       "originalEnd": string,   // For reschedule - original end time
       "newStart": string,     // For reschedule - new start time
       "newEnd": string        // For reschedule - new end time
     },
     "message": string,        // Human readable explanation
     "suggestions": [string],  // Array of suggestions/optimizations
     "conflicts": [string]     // Array of potential conflicts
   }

2. For calendar modifications:
   - Add event: Set action="add_event" and include title, startTime, endTime
   - Remove event: Set action="remove_event" and include title, startTime, endTime
   - Reschedule: Set action="reschedule" and include all time fields

3. All times should be in ISO 8601 format

4. Always validate:
   - No scheduling conflicts
   - Valid date/time formats
   - Timezone considerations
   - Calendar consistency

5. Include helpful suggestions and conflict warnings in the respective arrays

Remember to:
- Keep all responses in strict JSON format
- Parse natural language into proper datetime formats
- Handle timezone conversions automatically
- Proactively identify conflicts
- Learn from scheduling patterns
- Take initiative to optimize the calendar`,
			),
		},
	}

	chatSession := model.StartChat()
	return chatSession
}

func UpdateSchedule(session *genai.ChatSession, event *calendar.Event, action string) {
	// Convert times to user-friendly format
	startTime := formatDateTime(event.Start.DateTime)
	endTime := formatDateTime(event.End.DateTime)

	updateMessage := fmt.Sprintf(`Calendar Update:
	Action: %s
	Event: %s
	Time: %s to %s`,
		action,
		event.Summary,
		startTime,
		endTime,
	)

	session.History = append(session.History, &genai.Content{
		Parts: []genai.Part{
			genai.Text(updateMessage),
		},
		Role: "user",
	})
}

func SendGeminiMessage(chatSession *genai.ChatSession, message string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	message = time.Now().Format(time.RFC3339) + " : " + message
	defer cancel()

	resp, err := chatSession.SendMessage(ctx, genai.Text(message))
	if err != nil {
		log.Printf("Error sending message to Gemini: %v", err)
		return "", fmt.Errorf("failed to get AI response: %w", err)
	}

	var responseBuilder strings.Builder
	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				responseBuilder.WriteString(fmt.Sprintf("%v", part))
			}
		}
	}
	log.Println(responseBuilder.String())
	return responseBuilder.String(), nil
}

// Helper function to format datetime strings
func formatDateTime(datetime string) string {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		return datetime
	}
	return t.Format("Mon, Jan 2 at 3:04 PM")
}
