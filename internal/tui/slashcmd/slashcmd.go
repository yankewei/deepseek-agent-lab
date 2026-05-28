// Package slashcmd provides slash command metadata for the TUI chat interface.
package slashcmd

// Command describes a single slash command available in the chat input.
type Command struct {
	Name        string
	Description string
}

// All returns the full list of available slash commands.
func All() []Command {
	return []Command{
		{Name: "/clear", Description: "清除当前对话"},
		{Name: "/help", Description: "显示可用命令"},
		{Name: "/quit", Description: "退出应用"},
	}
}
