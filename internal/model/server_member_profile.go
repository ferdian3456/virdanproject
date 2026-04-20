package model

type ServerMemberProfile struct {
	Id            string  `json:"id"`
	ServerId      string  `json:"serverId"`
	UserId        string  `json:"userId"`
	Nickname      string  `json:"nickname"`
	Bio           *string `json:"bio"`
	AvatarImageId *string `json:"avatarImageId"`
	Audit
}

type ServerMemberProfileAvatarImage struct {
	Id        string `json:"id"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"objectKey"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	Audit
}

type ProfileHistoryResponse struct {
	Nickname    string  `json:"nickname"`
	Bio         *string `json:"bio"`
	AvatarImage *string `json:"avatarImage"` // URL of the avatar
}

type ServerMemberProfileUpdateRequest struct {
	Nickname      string  `json:"nickname"`
	Bio           *string `json:"bio"`
	AvatarImageId *string `json:"avatarImageId"`
}
