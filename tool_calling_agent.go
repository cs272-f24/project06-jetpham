package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/openai/openai-go"
)

func toolCallingAgent(setup Setup, prompt string, previousMessages []openai.ChatCompletionMessageParamUnion) (string, []openai.ChatCompletionMessageParamUnion) {
	// using the official example:
	//https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go

	systemPrompt := `
		You are a Retrieval Augmented Generation model that assists university students using course information.
	`
	messages := openai.F([]openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	})
	messages.Value = append(messages.Value, previousMessages...)
	messages.Value = append(messages.Value, openai.UserMessage(prompt))
	params := openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
		}),
	}

	params.Messages.Value = append(params.Messages.Value, previousMessages...)

	params.Messages.Value = append(params.Messages.Value, openai.UserMessage(prompt))

	params.Tools = openai.F([]openai.ChatCompletionToolParam{
		{
			Type: openai.F(openai.ChatCompletionToolTypeFunction),
			Function: openai.F(openai.FunctionDefinitionParam{
				Name:        openai.String("get_courses"),
				Description: openai.String("get course information"),
				Parameters: openai.F(openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt": map[string]string{
							"type": "string",
						},
					},
					"required": []string{"prompt"},
				}),
			}),
		},
		{
			Type: openai.F(openai.ChatCompletionToolTypeFunction),
			Function: openai.F(openai.FunctionDefinitionParam{
				Name:        openai.String("get_rate_my_professor_data"),
				Description: openai.String("get professor information from Rate My Professor"),
				Parameters: openai.F(openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{
							"type": "string",
						},
					},
					"required": []string{"name"},
				}),
			}),
		},
	})
	params.Model = openai.F(openai.ChatModelGPT4oMini)

	completion, err := setup.openAIClient.client.Chat.Completions.New(context.TODO(), params)
	if err != nil {
		log.Printf("Error creating chat completion: %v", err)
		return "", params.Messages.Value
	}

	toolCalls := completion.Choices[0].Message.ToolCalls

	// If there was not tool calls, crashout
	if len(toolCalls) == 0 {
		log.Printf("No function call")
		return completion.Choices[0].Message.Content, params.Messages.Value
	} else {
		log.Printf("Function call: %v\n", toolCalls[0].Function.Name)
		fmt.Printf("Function call: %v\n", toolCalls[0].Function.Name)
	}

	// If there was tool calls, continue
	params.Messages.Value = append(params.Messages.Value, completion.Choices[0].Message)
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "get_courses" {
			// Extract the prompt from the function call arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("Error unmarshalling arguments: %v", err)
				continue
			}
			prompt := args["prompt"].(string)

			log.Printf("%v(\"%s\")", toolCalls[0].Function.Name, prompt)

			// Call the getCourses function with the arguments requested by the model
			courses, err := getCourses(&setup, prompt)
			if err != nil {
				log.Printf("Error getting courses: %v", err)
				continue
			}
			coursesJSON, _ := json.Marshal(courses)
			params.Messages.Value = append(params.Messages.Value, openai.ToolMessage(toolCall.ID, string(coursesJSON)))
		} else if toolCall.Function.Name == "get_rate_my_professor_data" {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("Error unmarshalling arguments: %v", err)
				continue
			}
			names := args["names"].([]interface{})
			nameStrings := make([]string, len(names))
			for i, name := range names {
				nameStrings[i] = name.(string)
			}

			log.Printf("%v(\"%s\")", toolCalls[0].Function.Name, nameStrings)

			professorInfo, err := getManyProfessorsData(nameStrings)
			if err != nil {
				log.Printf("Error getting professor info: %v", err)
				continue
			}
			professorInfoJSON, _ := json.Marshal(professorInfo)
			log.Printf("Professor Info: %v", string(professorInfoJSON))
			params.Messages.Value = append(params.Messages.Value, openai.ToolMessage(toolCall.ID, string(professorInfoJSON)))
		}

		params.Messages.Value = append(params.Messages.Value, openai.SystemMessage(`
		Task: Generate an answer that corresponds to the provided question, mimicking the question's structure and format. Ensure the response is succinct, directly relevant to the query, and excludes any extraneous details.

		Reponce Format:
			[answer to the question]
			cited courses:
			[course details]
			
		Course Details: If relevant courses are involved in the answer, format them as follows:
		- Format: (subject_code)(course_number)-(section) title_short_desc by primary_instructor_full_name (relevant course details).
		- Ensure each course listed accurately responds to the original question.

		Response Guidelines:
		- Provide responses in plain text format, avoiding markdown.
		- List course details in a numbered format for clarity.
		- Ensure responce directory addresses the question
		`))
		params.Messages.Value = append(params.Messages.Value, openai.UserMessage(fmt.Sprintf("Prompt: %s", prompt)))
		completion, err = setup.openAIClient.client.Chat.Completions.New(context.TODO(), params)
		if err != nil {
			log.Printf("Error creating chat completion: %v", err)
			return "", params.Messages.Value
		}

		return completion.Choices[0].Message.Content, params.Messages.Value
	}
	// If there was no tool calls, return the completion this basically only happens if it calls a tool that doesn't exist
	return completion.Choices[0].Message.Content, params.Messages.Value
}
