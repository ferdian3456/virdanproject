package model

type ServerBannerImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	Audit
}
