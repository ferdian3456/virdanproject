package model

type ServerAvatarImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	Audit
}
