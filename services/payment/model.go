package payment

import "time"

type Page struct {
	NextCursor string `json:"nextCursor"`
}

type ServerPlusOrder struct {
	Id              string
	ServerId        string
	UserId          string
	ReferenceId     string
	XenditSessionId string
	XenditPaymentId string
	BaseIdr         int64
	TaxIdr          int64
	TotalIdr        int64
	Status          string
	PaidAt          *time.Time
	PlusExpiresAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       string
	UpdatedBy       string
}

type XenditWebhookEvent struct {
	Id          string
	EventId     string
	EventType   string
	ReferenceId string
	Payload     []byte
	Status      string
	ReceivedAt  time.Time
	ProcessedAt *time.Time
}

type PlusPriceResponse struct {
	BaseIdr  int64 `json:"baseIdr"`
	TaxIdr   int64 `json:"taxIdr"`
	TotalIdr int64 `json:"totalIdr"`
}

type PlusStatusResponse struct {
	Active       bool              `json:"active"`
	ExpiresAt    *time.Time        `json:"expiresAt"`
	DurationDays int               `json:"durationDays"`
	Price        PlusPriceResponse `json:"price"`
}

type PlusCheckoutResponse struct {
	OrderId    string `json:"orderId"`
	PaymentUrl string `json:"paymentUrl"`
}

type PlusOrderHistoryItem struct {
	Id            string     `json:"id"`
	ServerId      string     `json:"serverId"`
	ServerName    string     `json:"serverName"`
	TotalIdr      int64      `json:"totalIdr"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paidAt"`
	PlusExpiresAt *time.Time `json:"plusExpiresAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type PlusOrderHistoryResponse struct {
	Data []PlusOrderHistoryItem `json:"data"`
	Page Page                   `json:"page"`
}

type PlusOrderDetailResponse struct {
	Id            string     `json:"id"`
	ServerId      string     `json:"serverId"`
	ServerName    string     `json:"serverName"`
	ReferenceId   string     `json:"referenceId"`
	BaseIdr       int64      `json:"baseIdr"`
	TaxIdr        int64      `json:"taxIdr"`
	TotalIdr      int64      `json:"totalIdr"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paidAt"`
	PlusExpiresAt *time.Time `json:"plusExpiresAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type PlusOrderCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	Id        string    `json:"id"`
}
