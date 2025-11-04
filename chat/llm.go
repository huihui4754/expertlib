package chat

type LLMChatWithFunCall interface {
	Chat(string) string
}
