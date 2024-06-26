package anthropic

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hunterjsb/super-claude/utils"
)

type Handler struct {
	Tools *[]Tool
}

func (h *Handler) ConverseHttp(w http.ResponseWriter, r *http.Request) {
	convo := Conversation{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "ERROR: Invalid data: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &convo)
	if err != nil {
		http.Error(w, "ERROR: Invalid values, must be of type Conversation: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Converse
	req := &Request{Model: Opus, Messages: convo, MaxTokens: 2048, System: SYS_PROMPT, Tools: *h.Tools}
	convo.talkHttp(req, w)
}

func (convo *Conversation) talkHttp(req *Request, w http.ResponseWriter) {
	resp, err := req.Post()
	if err != nil {
		errMsg := utils.Csprintf("red", "Error making request: %s", err.Error())
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	var responseMsg string
	for _, cont := range resp.Content {
		if cont.Type == MessageResp || cont.Type == Text {
			thoughts, message := parseThoughts(cont.Text)
			if thoughts != "" {
				responseMsg += utils.Csprintf(claudeThoughtsColor, "\n*Thinking* %s\n", thoughts)
			}
			if message != "" {
				responseMsg += utils.Csprintf(claudeColor, "Claude:\n")
				responseMsg += utils.Csprintf(claudeResponseColor, "%s\n", message)
			}
			convo.appendMsg(Message{Role: Assistant, Content: wrapContent(&cont)})
		} else if cont.Type == ToolUse {
			convo.useToolHttp(cont, &responseMsg)
			req.Messages = *convo
			convo.talkHttp(req, w) // Recursively call talk to handle the next step
		} else {
			errMsg := utils.Csprintf("red", "Error: Unknown response type %s", cont.Type)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
	}
	w.Write([]byte(responseMsg))
}

func (convo *Conversation) useToolHttp(input Content, responseMsg *string) {
	*responseMsg += utils.Csprintf(toolRequestColor, "Claude wants to use tool: '%s' with inputs: %v", input.Name, input.Input)
	toolResp := ToolMap[input.Name](input.Input)
	currentToolUId = input.Id // dangerous?
	if (*convo)[len(*convo)-1].Role == Assistant {
		(*convo)[len(*convo)-1].Content = append((*convo)[len(*convo)-1].Content, input) // append to message content instead of conversation
	} else {
		convo.appendMsg(Message{Role: Assistant, Content: makeToolUseContent(&input)})
	}
	*responseMsg += utils.Csprintf(toolResponseColor, "Used tool '%s' and got response: %v", input.Name, toolResp.Content)
	convo.appendMsg(Message{Role: User, Content: makeToolResponseContent(&toolResp)})
}
