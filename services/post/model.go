package post

import "time"

type Page struct {
	NextCursor string `json:"nextCursor"`
}

type AuthorStatus string

const (
	AuthorStatusActive      AuthorStatus = "active"
	AuthorStatusUserLeft    AuthorStatus = "user_left"
	AuthorStatusUserDeleted AuthorStatus = "user_deleted"
)

type AuthorIdentityResponse struct {
	UserId    string       `json:"userId"`
	Nickname  string       `json:"nickname"`
	Username  string       `json:"username"`
	AvatarUrl *string      `json:"avatarUrl"`
	Status    AuthorStatus `json:"status"`
}

type ServerPost struct {
	Id          string
	ServerId    string
	AuthorId    string
	PostImageId *string
	PostVideoId *string
	Caption     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

type ServerPostImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	Width     int
	Height    int
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerPostVideo struct {
	Id                 string
	Bucket             string
	ObjectKey          string
	MimeType           string
	Size               int64
	Duration           int
	Width              int
	Height             int
	ThumbnailObjectKey string
	Mirrored           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CreatedBy          string
	UpdatedBy          string
}

type ServerPostLike struct {
	Id        string
	PostId    string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerPostComment struct {
	Id        string
	PostId    string
	AuthorId  string
	ParentId  *string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerPostSave struct {
	Id        string
	PostId    string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerPostUpdateCaptionRequest struct {
	Caption string `json:"caption"`
}

type ServerPostCursor struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServerPostListResponse struct {
	Data []ServerPostResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerPostResponse struct {
	Id           string                 `json:"id"`
	ServerId     string                 `json:"serverId"`
	Caption      string                 `json:"caption"`
	ImageUrl     *string                `json:"imageUrl"`
	VideoUrl     *string                `json:"videoUrl,omitempty"`
	ThumbnailUrl *string                `json:"thumbnailUrl,omitempty"`
	MediaType    string                 `json:"mediaType"`
	MediaWidth   *int                   `json:"mediaWidth,omitempty"`
	MediaHeight  *int                   `json:"mediaHeight,omitempty"`
	Mirrored     *bool                  `json:"mirrored,omitempty"`
	Author       AuthorIdentityResponse `json:"author"`
	LikeCount    int                    `json:"likeCount"`
	CommentCount int                    `json:"commentCount"`
	UserLiked    bool                   `json:"userLiked"`
	UserSaved    bool                   `json:"userSaved"`
	SavedAt      *time.Time             `json:"savedAt,omitempty"`
	IsOwner      bool                   `json:"isOwner"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

type PostLikeResponse struct {
	PostId    string `json:"postId"`
	UserLiked bool   `json:"userLiked"`
	LikeCount int    `json:"likeCount"`
}

type ServerCommentCreateRequest struct {
	Content  string  `json:"content"`
	ParentId *string `json:"parentId"`
}

type ServerCommentCursor struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServerCommentListResponse struct {
	Data []ServerCommentResponse `json:"data"`
	Page Page                    `json:"page"`
}

type ServerCommentResponse struct {
	Id        string                 `json:"id"`
	PostId    string                 `json:"postId"`
	ParentId  *string                `json:"parentId"`
	Content   string                 `json:"content"`
	Author    AuthorIdentityResponse `json:"author"`
	IsOwner   bool                   `json:"isOwner"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

type PostSaveResponse struct {
	PostId    string `json:"postId"`
	UserSaved bool   `json:"userSaved"`
}

type SavedPostCursor struct {
	SavedAt time.Time `json:"savedAt"`
	PostId  string    `json:"postId"`
}
