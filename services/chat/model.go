package chat

import "time"

type Page struct {
	NextCursor string `json:"nextCursor"`
}

type DmConversation struct {
	Id            string
	ServerId      string
	UserLow       string
	UserHigh      string
	LastMessageAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     string
	UpdatedBy     string
}

type DmMessage struct {
	Id              string
	ConversationId  string
	SenderId        string
	Type            string
	Content         string
	ClientMessageId string
	CreatedAt       time.Time
}

type DmIdentity struct {
	Nickname  string  `json:"nickname"`
	Username  string  `json:"username"`
	AvatarUrl *string `json:"avatarUrl"`
}

type GetOrCreateConversationRequest struct {
	PeerUserId string `json:"peerUserId"`
}

type SendMessageRequest struct {
	Content         string `json:"content"`
	ClientMessageId string `json:"clientMessageId"`
}

type MarkReadRequest struct {
	LastReadMessageId *string `json:"lastReadMessageId"`
}

type DmMessageResponse struct {
	Id              string     `json:"id"`
	ConversationId  string     `json:"conversationId"`
	SenderId        string     `json:"senderId"`
	Sender          DmIdentity `json:"sender"`
	Type            string     `json:"type"`
	Content         string     `json:"content"`
	ClientMessageId string     `json:"clientMessageId"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type DmMessageListResponse struct {
	Data []DmMessageResponse `json:"data"`
	Page Page                `json:"page"`
}

type DmConversationResponse struct {
	Id                 string     `json:"id"`
	ServerId           string     `json:"serverId"`
	PeerUserId         string     `json:"peerUserId"`
	Peer               DmIdentity `json:"peer"`
	UnreadCount        int        `json:"unreadCount"`
	IsOnline           bool       `json:"isOnline"`
	LastMessagePreview *string    `json:"lastMessagePreview"`
	LastMessageAt      *time.Time `json:"lastMessageAt"`
}

type DmConversationListResponse struct {
	Data []DmConversationResponse `json:"data"`
	Page Page                     `json:"page"`
}

type DmMemberResponse struct {
	UserId             string     `json:"userId"`
	Identity           DmIdentity `json:"identity"`
	ConversationId     *string    `json:"conversationId"`
	UnreadCount        int        `json:"unreadCount"`
	LastMessagePreview *string    `json:"lastMessagePreview"`
	LastMessageAt      *time.Time `json:"lastMessageAt"`
}

type DmMemberListResponse struct {
	Data []DmMemberResponse `json:"data"`
	Page Page               `json:"page"`
}

type DmMessageCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	Id        string    `json:"id"`
}

type DmConversationCursor struct {
	LastMessageAt  time.Time `json:"lastMessageAt"`
	ConversationId string    `json:"conversationId"`
}

type DmMemberCursor struct {
	Nickname string `json:"nickname"`
	UserId   string `json:"userId"`
}

type WsInboundTyping struct {
	Type    string `json:"type"`
	Payload struct {
		ConversationId string `json:"conversationId"`
		IsTyping       bool   `json:"isTyping"`
	} `json:"payload"`
}

type WsTypingPayload struct {
	ConversationId string `json:"conversationId"`
	UserId         string `json:"userId"`
	IsTyping       bool   `json:"isTyping"`
}

type WsReadPayload struct {
	ConversationId string    `json:"conversationId"`
	UserId         string    `json:"userId"`
	LastReadAt     time.Time `json:"lastReadAt"`
}

type WsPresencePayload struct {
	UserId string `json:"userId"`
	Online bool   `json:"online"`
}
