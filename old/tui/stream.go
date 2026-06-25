package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"

	"github.com/ZiplEix/talos/storage"
	"github.com/ZiplEix/talos/tools"
)

func (m *Model) sendUserMessage(input string) tea.Cmd {
	m.chatMessages = append(m.chatMessages, ChatMessage{Role: "user", Content: input})
	m.oaiMessages = append(m.oaiMessages, openai.UserMessage(input))
	m.state = StateLoading
	m.streamBuf.Reset()
	m.updateViewport()
	m.viewport.GotoBottom()
	return m.startStreaming()
}

func (m *Model) startStreaming() tea.Cmd {
	client := m.client
	messages := make([]openai.ChatCompletionMessageParamUnion, len(m.oaiMessages))
	copy(messages, m.oaiMessages)
	modelName := m.settings.CurrentModel
	p := m.Program
	ctx := m.cancelCtx

	return func() tea.Msg {
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(modelName),
			Messages: messages,
			Tools:    tools.GetRegisteredTools(),
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		var contentBuf strings.Builder

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					contentBuf.WriteString(delta.Content)
					if p != nil {
						p.Send(streamTokenMsg(delta.Content))
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			return streamErrMsg{err: err}
		}

		var toolCalls []openai.ChatCompletionMessageToolCallUnion
		if len(acc.Choices) > 0 {
			toolCalls = acc.Choices[0].Message.ToolCalls
		}

		return streamDoneMsg{
			content:   contentBuf.String(),
			toolCalls: toolCalls,
		}
	}
}

func (m *Model) handleStreamDone(msg streamDoneMsg) tea.Cmd {
	if len(msg.toolCalls) > 0 {
		var tcs []openai.ChatCompletionMessageToolCallUnionParam
		for _, tc := range msg.toolCalls {
			tcs = append(tcs, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID:   tc.ID,
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				},
			})
		}
		assistantParam := openai.ChatCompletionAssistantMessageParam{
			ToolCalls: tcs,
		}
		if msg.content != "" {
			assistantParam.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: param.NewOpt(msg.content),
			}
		}
		m.oaiMessages = append(m.oaiMessages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &assistantParam,
		})

		if msg.content != "" {
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:    "thought",
				Content: msg.content,
			})
		}

		for _, tc := range msg.toolCalls {
			paramVal := tools.GetToolParamValue(tc.Function.Name, tc.Function.Arguments)
			m.chatMessages = append(m.chatMessages, ChatMessage{
				Role:    "tool",
				Content: fmt.Sprintf("%s(%s)", tc.Function.Name, paramVal),
			})
		}

		_ = storage.SaveConversation(m.convID, m.oaiMessages)

		m.toolQueue = msg.toolCalls
		m.toolResults = nil
		m.toolQueueIndex = 0
		m.state = StateExecutingTools
		m.streamBuf.Reset()
		m.updateViewport()
		m.viewport.GotoBottom()
		return m.executeNextToolCmd()
	}

	m.state = StateInput
	content := msg.content
	if content == "" {
		content = m.streamBuf.String()
	}

	if content != "" {
		m.chatMessages = append(m.chatMessages, ChatMessage{Role: "assistant", Content: content})
		m.oaiMessages = append(m.oaiMessages, openai.AssistantMessage(content))
	}

	m.streamBuf.Reset()
	_ = storage.SaveConversation(m.convID, m.oaiMessages)
	m.updateViewport()
	m.viewport.GotoBottom()
	return nil
}

func (m *Model) executeNextToolCmd() tea.Cmd {
	if m.toolQueueIndex >= len(m.toolQueue) {
		return nil
	}
	tc := m.toolQueue[m.toolQueueIndex]
	ctx := m.cancelCtx
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return streamErrMsg{err: fmt.Errorf("cancelled")}
		default:
			result, toolCallID := tools.HandleToolCall(tc)
			return toolResultMsg{
				result:     result,
				toolCallID: toolCallID,
				toolName:   tc.Function.Name,
			}
		}
	}
}